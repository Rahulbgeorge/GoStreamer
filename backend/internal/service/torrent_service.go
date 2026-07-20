package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"streamingplayer/internal/config"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
	"streamingplayer/pkg/fileparser"
)

// TorrentStatus represents the current state of an active torrent download.
type TorrentStatus struct {
	MediaID        string  `json:"media_id"`
	Title          string  `json:"title"`
	Status         string  `json:"status"`
	TotalBytes     int64   `json:"total_size"`
	CompletedBytes int64   `json:"completed_bytes"`
	ProgressPct    float64 `json:"progress_pct"`
	DownloadRate   float64 `json:"download_rate_bps"`
	Peers          int     `json:"peers"`
}

type TorrentTarget struct {
	Title string `json:"title"`
	Size  string `json:"size"`
	Link  string `json:"link"`
}

type TorrentService interface {
	Close()
	AddMagnet(ctx context.Context, magnetURI string) (*model.Download, error)
	GetStatus(mediaID string) (*TorrentStatus, error)
	ListActive() []TorrentStatus
	CancelTorrent(mediaID string) error
	ScanHTML(ctx context.Context, pageURL string) ([]TorrentTarget, error)
}

type activeTorrent struct {
	downloadID       string
	title            string
	hash             string
	transmissionID   string
	metadataResolved bool
	cancelFunc       context.CancelFunc
}

type torrentService struct {
	config         *config.Config
	repo           repository.MediaRepository
	prefRepo       repository.PreferenceRepository
	downloadRepo   repository.DownloadRepository
	scannerService ScannerService
	mu             sync.Mutex
	active         map[string]*activeTorrent // keyed by media ID
}

func NewTorrentService(
	cfg *config.Config,
	repo repository.MediaRepository,
	prefRepo repository.PreferenceRepository,
	downloadRepo repository.DownloadRepository,
	scannerService ScannerService,
) (TorrentService, error) {
	s := &torrentService{
		config:         cfg,
		repo:           repo,
		prefRepo:       prefRepo,
		downloadRepo:   downloadRepo,
		scannerService: scannerService,
		active:         make(map[string]*activeTorrent),
	}

	// Verify transmission-remote is available on the system
	if err := exec.Command("transmission-remote", "-l").Run(); err != nil {
		slog.Warn("transmission-remote check failed. Ensure transmission-daemon is installed and running.", "err", err)
	}

	go s.resumeActiveTorrents()

	return s, nil
}

func (s *torrentService) getMediaDir() string {
	pref, err := s.prefRepo.Get("homedir")
	if err == nil && pref != nil && pref.Value != "" {
		return pref.Value
	}
	return s.config.MediaDir
}

func (s *torrentService) getTorrentDownloadDir() string {
	baseDir := s.getMediaDir()
	torrentDir := filepath.Join(baseDir, "torrent-download")
	if err := os.MkdirAll(torrentDir, 0777); err != nil {
		slog.Error("Failed to create torrent-download directory", "path", torrentDir, "err", err)
	}
	return torrentDir
}

func (s *torrentService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Cancel all active tracking loops
	for _, at := range s.active {
		at.cancelFunc()
	}
}

func (s *torrentService) AddMagnet(ctx context.Context, magnetURI string) (*model.Download, error) {
	hash := getTorrentHash(magnetURI)
	if hash == "" {
		return nil, fmt.Errorf("could not extract valid info hash from magnet link")
	}

	// Default values before metadata is resolved
	title := "Fetching torrent metadata..."
	if u, err := url.Parse(magnetURI); err == nil {
		if dn := u.Query().Get("dn"); dn != "" {
			title = dn
		}
	}
	if title == "Fetching torrent metadata..." {
		re := regexp.MustCompile(`(?i)dn=([^&]+)`)
		matches := re.FindStringSubmatch(magnetURI)
		if len(matches) > 1 {
			if decoded, err := url.QueryUnescape(matches[1]); err == nil {
				title = decoded
			} else {
				title = matches[1]
			}
		}
	}

	downloadID := uuid.New().String()
	torrentDir := s.getTorrentDownloadDir()

	dl := &model.Download{
		ID:            downloadID,
		Title:         title,
		Status:        model.DownloadStatusDownloading,
		Type:          model.DownloadTypeTorrent,
		Progress:      0,
		TotalSize:     0,
		CompletedSize: 0,
		DestPath:      filepath.Join(torrentDir, "pending-"+hash),
	}

	// Clean/truncate magnet link to the first & for transmission-remote compatibility
	transmissionURI := magnetURI
	if idx := strings.Index(transmissionURI, "&"); idx != -1 {
		transmissionURI = transmissionURI[:idx]
	}

	// Add to Transmission
	cmd := exec.Command("transmission-remote", "-a", transmissionURI, "-w", torrentDir)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to add torrent to transmission: %w", err)
	}

	if err := s.downloadRepo.Create(dl); err != nil {
		return nil, fmt.Errorf("create download record: %w", err)
	}

	trackCtx, cancelFunc := context.WithCancel(context.Background())

	s.mu.Lock()
	s.active[downloadID] = &activeTorrent{
		downloadID:       downloadID,
		title:            title,
		hash:             hash,
		metadataResolved: false,
		cancelFunc:       cancelFunc,
	}
	s.mu.Unlock()

	go s.trackTorrentDownload(trackCtx, downloadID, hash, dl)

	return dl, nil
}

func (s *torrentService) GetStatus(downloadID string) (*TorrentStatus, error) {
	s.mu.Lock()
	at, exists := s.active[downloadID]
	s.mu.Unlock()

	if !exists {
		dl, err := s.downloadRepo.FindByID(downloadID)
		if err != nil {
			return nil, fmt.Errorf("lookup download: %w", err)
		}
		if dl == nil {
			return nil, fmt.Errorf("download not found: %s", downloadID)
		}
		return &TorrentStatus{
			MediaID:        dl.ID,
			Title:          dl.Title,
			Status:         string(dl.Status),
			TotalBytes:     dl.TotalSize,
			CompletedBytes: dl.CompletedSize,
			ProgressPct:    dl.Progress,
			DownloadRate:   dl.DownloadSpeed,
		}, nil
	}

	dl, err := s.downloadRepo.FindByID(downloadID)
	if err != nil || dl == nil {
		return nil, fmt.Errorf("lookup download: %w", err)
	}

	if at.transmissionID == "" {
		return &TorrentStatus{
			MediaID: at.downloadID,
			Title:   at.title,
			Status:  "pending",
		}, nil
	}

	jobs, err := s.queryTransmissionList()
	if err != nil {
		return nil, fmt.Errorf("query transmission list: %w", err)
	}

	job, found := jobs[at.transmissionID]
	if !found {
		return &TorrentStatus{
			MediaID: at.downloadID,
			Title:   at.title,
			Status:  string(dl.Status),
		}, nil
	}

	peers := getPeerCount(at.transmissionID)

	return &TorrentStatus{
		MediaID:        at.downloadID,
		Title:          at.title,
		Status:         string(dl.Status),
		TotalBytes:     dl.TotalSize,
		CompletedBytes: int64(float64(dl.TotalSize) * job.ProgressPct / 100.0),
		ProgressPct:    job.ProgressPct,
		DownloadRate:   job.DownloadRate,
		Peers:          peers,
	}, nil
}

func (s *torrentService) ListActive() []TorrentStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobs, err := s.queryTransmissionList()
	if err != nil {
		return nil
	}

	var result []TorrentStatus
	for _, at := range s.active {
		dl, _ := s.downloadRepo.FindByID(at.downloadID)
		status := TorrentStatus{
			MediaID: at.downloadID,
			Title:   at.title,
			Status:  "pending",
		}
		if dl != nil {
			status.Status = string(dl.Status)
		}

		if at.transmissionID != "" {
			if job, found := jobs[at.transmissionID]; found {
				status.ProgressPct = job.ProgressPct
				status.DownloadRate = job.DownloadRate
				if dl != nil {
					status.TotalBytes = dl.TotalSize
					status.CompletedBytes = int64(float64(dl.TotalSize) * job.ProgressPct / 100.0)
				}
				status.Peers = getPeerCount(at.transmissionID)
			}
		}

		result = append(result, status)
	}
	return result
}

func (s *torrentService) CancelTorrent(downloadID string) error {
	s.mu.Lock()
	at, exists := s.active[downloadID]
	if exists {
		delete(s.active, downloadID)
	}
	s.mu.Unlock()

	dl, err := s.downloadRepo.FindByID(downloadID)
	if err != nil {
		return err
	}
	if dl == nil {
		return fmt.Errorf("download not found: %s", downloadID)
	}

	if exists {
		at.cancelFunc()
	}

	if at != nil && at.transmissionID != "" {
		_ = exec.Command("transmission-remote", "-t", at.transmissionID, "-rad").Run()
	}

	dl.Status = model.DownloadStatusCancelled
	_ = s.downloadRepo.Update(dl)

	slog.Info("Torrent download cancelled and cleaned up", "downloadID", downloadID, "title", dl.Title)
	return nil
}

func (s *torrentService) trackTorrentDownload(ctx context.Context, downloadID string, hash string, dl *model.Download) {
	ticker := time.NewTicker(time.Duration(s.config.DownloadUpdateInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			at, exists := s.active[downloadID]
			s.mu.Unlock()
			if !exists {
				return
			}

			// 1. Resolve transmissionID if not done yet
			if at.transmissionID == "" {
				jobs, err := s.queryTransmissionList()
				if err == nil {
					for id := range jobs {
						h, errH := getTorrentHashFromInfo(id)
						if errH == nil && strings.ToLower(h) == strings.ToLower(hash) {
							s.mu.Lock()
							at.transmissionID = id
							s.mu.Unlock()
							break
						}
					}
				}
			}

			if at.transmissionID == "" {
				continue
			}

			// 2. Resolve metadata if not done yet
			if !at.metadataResolved {
				files, err := getTorrentFiles(at.transmissionID)
				if err == nil && len(files) > 0 {
					var largestFile *TorrentFile
					var maxSize int64
					for i := range files {
						if files[i].Size > maxSize && isVideoFile(files[i].Name) {
							maxSize = files[i].Size
							largestFile = &files[i]
						}
					}

					if largestFile != nil {
						filename := filepath.Base(largestFile.Name)
						meta := fileparser.ParseFilename(filename)
						finalPath := filepath.Join(s.getTorrentDownloadDir(), largestFile.Name)

						dl.Title = meta.Title
						dl.DestPath = finalPath
						dl.TotalSize = largestFile.Size
						_ = s.downloadRepo.Update(dl)

						s.mu.Lock()
						at.title = meta.Title
						at.metadataResolved = true
						s.mu.Unlock()
						slog.Info("Torrent metadata resolved in background", "title", dl.Title, "file", filename)
					}
				}
			}

			// 3. Track progress and finish download
			if at.metadataResolved {
				jobs, err := s.queryTransmissionList()
				if err == nil {
					job, found := jobs[at.transmissionID]
					if found {
						dl.Progress = job.ProgressPct
						dl.CompletedSize = int64(float64(dl.TotalSize) * job.ProgressPct / 100.0)
						dl.DownloadSpeed = job.DownloadRate
						
						remainingBytes := dl.TotalSize - dl.CompletedSize
						if dl.DownloadSpeed > 0 && remainingBytes > 0 {
							etaSecs := int64(float64(remainingBytes) / dl.DownloadSpeed)
							dl.ETA = fmt.Sprintf("%ds", etaSecs)
						} else {
							dl.ETA = ""
						}

						_ = s.downloadRepo.Update(dl)

						if job.ProgressPct >= 100.0 {
							slog.Info("Torrent download complete", "title", dl.Title, "path", dl.DestPath)

							dl.Status = model.DownloadStatusCompleted
							dl.Progress = 100.0
							_ = s.downloadRepo.Update(dl)

							// Clean up Transmission job (remove from Transmission daemon without deleting downloaded data)
							_ = exec.Command("transmission-remote", "-t", at.transmissionID, "-r").Run()

							s.removeActive(downloadID)
							slog.Info("Torrent download complete, triggering directory scan", "title", dl.Title)

							// Automatically scan folder to ingest the file and extract thumbnails
							go s.scannerService.ScanDirectory(context.Background())
							return
						}
					}
				}
			}
		}
	}
}

func (s *torrentService) removeActive(mediaID string) {
	s.mu.Lock()
	delete(s.active, mediaID)
	s.mu.Unlock()
}

func (s *torrentService) resumeActiveTorrents() {
	allDl, err := s.downloadRepo.FindAll()
	if err != nil {
		slog.Error("Failed to lookup downloading torrents on startup", "err", err)
		return
	}

	var downloading []model.Download
	for _, dl := range allDl {
		if dl.Status == model.DownloadStatusDownloading && dl.Type == model.DownloadTypeTorrent {
			downloading = append(downloading, dl)
		}
	}

	jobs, err := s.queryTransmissionList()
	if err != nil {
		slog.Error("Failed to query transmission list on startup", "err", err)
		return
	}

	for _, dlVal := range downloading {
		dl := dlVal
		var matchedJob *TransmissionJob
		mTitle := strings.ToLower(dl.Title)
		for _, job := range jobs {
			jobTitle := strings.ToLower(job.Title)
			if mTitle != "" && (strings.Contains(jobTitle, mTitle) || strings.Contains(mTitle, jobTitle)) {
				matchedJob = job
				break
			}
		}

		if matchedJob != nil {
			hash, _ := getTorrentHashFromInfo(matchedJob.ID)
			trackCtx, cancelFunc := context.WithCancel(context.Background())
			at := &activeTorrent{
				downloadID:       dl.ID,
				title:            dl.Title,
				hash:             hash,
				transmissionID:   matchedJob.ID,
				metadataResolved: dl.DestPath != "" && !strings.Contains(dl.DestPath, "pending-"),
				cancelFunc:       cancelFunc,
			}
			s.mu.Lock()
			s.active[dl.ID] = at
			s.mu.Unlock()

			go s.trackTorrentDownload(trackCtx, dl.ID, hash, &dl)
			slog.Info("Resumed tracking torrent download on startup", "title", dl.Title, "transmissionID", matchedJob.ID)
		} else {
			dl.Status = model.DownloadStatusFailed
			_ = s.downloadRepo.Update(&dl)
		}
	}
}

type TransmissionJob struct {
	ID           string
	ProgressPct  float64
	DownloadRate float64
	Status       string
	Title        string
}

func (s *torrentService) queryTransmissionList() (map[string]*TransmissionJob, error) {
	cmd := exec.Command("transmission-remote", "-l")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) == 0 {
		return nil, nil
	}

	headerIdx := -1
	for idx, line := range lines {
		if strings.Contains(line, "ID") && strings.Contains(line, "Done") && strings.Contains(line, "Status") {
			headerIdx = idx
			break
		}
	}
	if headerIdx == -1 {
		return nil, fmt.Errorf("could not find header in transmission-remote -l output")
	}

	header := lines[headerIdx]
	idxID := strings.Index(header, "ID")
	idxDone := strings.Index(header, "Done")
	idxHave := strings.Index(header, "Have")
	idxDown := strings.Index(header, "Down")
	idxRatio := strings.Index(header, "Ratio")
	idxStatus := strings.Index(header, "Status")
	idxName := strings.Index(header, "Name")

	jobs := make(map[string]*TransmissionJob)

	for _, line := range lines[headerIdx+1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Sum:") {
			continue
		}

		getSlice := func(start, end int) string {
			if start < 0 || start >= len(line) {
				return ""
			}
			if end < 0 || end > len(line) {
				return strings.TrimSpace(line[start:])
			}
			return strings.TrimSpace(line[start:end])
		}

		idParts := strings.Fields(getSlice(idxID, idxDone))
		if len(idParts) == 0 {
			continue
		}
		id := idParts[0]

		doneParts := strings.Fields(getSlice(idxDone, idxHave))
		doneStr := ""
		if len(doneParts) > 0 {
			doneStr = doneParts[0]
		}

		downParts := strings.Fields(getSlice(idxDown, idxRatio))
		downStr := ""
		if len(downParts) > 0 {
			downStr = downParts[0]
		}

		statusParts := strings.Fields(getSlice(idxStatus, idxName))
		status := ""
		if len(statusParts) > 0 {
			status = strings.Join(statusParts, " ")
		}

		name := strings.TrimSpace(line[idxName:])

		doneStr = strings.TrimSuffix(doneStr, "%")
		pct, _ := strconv.ParseFloat(doneStr, 64)
		downRate := parseSpeedBps(downStr)

		jobs[id] = &TransmissionJob{
			ID:           id,
			ProgressPct:  pct,
			DownloadRate: downRate,
			Status:       status,
			Title:        name,
		}
	}

	return jobs, nil
}

type TorrentFile struct {
	Index int
	Done  float64
	Size  int64
	Name  string
}

func getTorrentFiles(transmissionID string) ([]TorrentFile, error) {
	cmd := exec.Command("transmission-remote", "-t", transmissionID, "-f")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	var files []TorrentFile
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "files):") || strings.HasPrefix(line, "idx") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		idxStr := strings.TrimSuffix(fields[0], ":")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			continue
		}
		donePct := 0.0
		doneStr := strings.TrimSuffix(fields[1], "%")
		if dVal, errD := strconv.ParseFloat(doneStr, 64); errD == nil {
			donePct = dVal
		}

		sizeStr := fields[3]
		sizeUnit := fields[4]
		sizeBytes := parseSizeToBytes(sizeStr, sizeUnit)

		name := strings.Join(fields[6:], " ")

		files = append(files, TorrentFile{
			Index: idx,
			Done:  donePct,
			Size:  sizeBytes,
			Name:  name,
		} )
	}
	return files, nil
}

func parseSizeToBytes(sizeStr string, unit string) int64 {
	val, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0
	}
	switch strings.ToUpper(unit) {
	case "KB", "KIB":
		return int64(val * 1024)
	case "MB", "MIB":
		return int64(val * 1024 * 1024)
	case "GB", "GIB":
		return int64(val * 1024 * 1024 * 1024)
	case "TB", "TIB":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	}
	return int64(val)
}

func getPeerCount(transmissionID string) int {
	cmd := exec.Command("transmission-remote", "-t", transmissionID, "-i")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	re := regexp.MustCompile(`(?i)Peers:\s+connected\s+with\s+(\d+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		val, _ := strconv.Atoi(matches[1])
		return val
	}
	return 0
}

func getTorrentHash(magnetURI string) string {
	u, err := url.Parse(magnetURI)
	if err != nil {
		return ""
	}
	xt := u.Query().Get("xt")
	if !strings.HasPrefix(xt, "urn:btih:") {
		return ""
	}
	hash := strings.TrimPrefix(xt, "urn:btih:")
	return strings.ToLower(hash)
}

func parseSpeedBps(speedStr string) float64 {
	if speedStr == "0" || speedStr == "" {
		return 0
	}
	re := regexp.MustCompile(`(?i)([\d\.]+)\s*([a-z]*)`)
	matches := re.FindStringSubmatch(speedStr)
	if len(matches) < 2 {
		return 0
	}
	val, _ := strconv.ParseFloat(matches[1], 64)
	unit := ""
	if len(matches) > 2 {
		unit = matches[2]
	}
	switch strings.ToLower(unit) {
	case "kb/s", "kbps", "k":
		return val * 1000
	case "mb/s", "mbps", "m":
		return val * 1000000
	case "gb/s", "gbps", "g":
		return val * 1000000000
	}
	return val
}

func getTorrentHashFromInfo(transmissionID string) (string, error) {
	cmd := exec.Command("transmission-remote", "-t", transmissionID, "-i")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`(?i)Hash:\s+([a-f0-9]{32,40})`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		return strings.ToLower(matches[1]), nil
	}
	return "", fmt.Errorf("hash not found in info")
}

// ScanHTML fetches the HTML from pageURL and extracts all magnet links.
func (s *torrentService) ScanHTML(ctx context.Context, pageURL string) ([]TorrentTarget, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", pageURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	htmlContent := string(bodyBytes)

	// Regex to match anchor tags containing magnet links
	anchorRegex := regexp.MustCompile(`(?i)<a\s+[^>]*href=["'](magnet:\?[^"']+)["'][^>]*>([\s\S]*?)<\/a>`)
	matches := anchorRegex.FindAllStringSubmatch(htmlContent, -1)

	var targets []TorrentTarget
	for i, match := range matches {
		magnetURL := strings.ReplaceAll(match[1], "&amp;", "&")
		innerContent := match[2]

		resolvedTitle := ""
		if parsedMagnet, err := url.Parse(magnetURL); err == nil {
			dn := parsedMagnet.Query().Get("dn")
			if dn != "" {
				resolvedTitle = dn
			}
		}

		size := "Unknown Size"
		sizeRegex := regexp.MustCompile(`(?i)class=["']btn-size["'][^>]*>([^<]+)`)
		if sizeMatch := sizeRegex.FindStringSubmatch(innerContent); len(sizeMatch) > 1 {
			size = strings.TrimSpace(sizeMatch[1])
		}

		if resolvedTitle == "" {
			tagRegex := regexp.MustCompile(`<[^>]*>`)
			cleaned := tagRegex.ReplaceAllString(innerContent, " ")
			resolvedTitle = strings.TrimSpace(strings.Join(strings.Fields(cleaned), " "))
			if resolvedTitle == "" {
				resolvedTitle = fmt.Sprintf("Torrent Link %d", i+1)
			}
		}

		targets = append(targets, TorrentTarget{
			Title: resolvedTitle,
			Size:  size,
			Link:  magnetURL,
		})
	}

	return targets, nil
}
