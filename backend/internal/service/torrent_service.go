package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
	TotalBytes     int64   `json:"total_bytes"`
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
	AddMagnet(ctx context.Context, magnetURI string) (*model.Media, error)
	GetStatus(mediaID string) (*TorrentStatus, error)
	ListActive() []TorrentStatus
	CancelTorrent(mediaID string) error
	ScanHTML(ctx context.Context, pageURL string) ([]TorrentTarget, error)
}

type activeTorrent struct {
	media            *model.Media
	hash             string
	transmissionID   string
	metadataResolved bool
	cancelFunc       context.CancelFunc
}

type torrentService struct {
	config *config.Config
	repo   repository.MediaRepository
	mu     sync.Mutex
	active map[string]*activeTorrent // keyed by media ID
}

func NewTorrentService(cfg *config.Config, repo repository.MediaRepository) (TorrentService, error) {
	s := &torrentService{
		config: cfg,
		repo:   repo,
		active: make(map[string]*activeTorrent),
	}

	// Verify transmission-remote is available on the system
	if err := exec.Command("transmission-remote", "-l").Run(); err != nil {
		slog.Warn("transmission-remote check failed. Ensure transmission-daemon is installed and running.", "err", err)
	}

	go s.resumeActiveTorrents()

	return s, nil
}

func (s *torrentService) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Cancel all active tracking loops
	for _, at := range s.active {
		at.cancelFunc()
	}
}

func (s *torrentService) AddMagnet(ctx context.Context, magnetURI string) (*model.Media, error) {
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

	meta := fileparser.ParseFilename(title)
	mediaID := uuid.New().String()

	language := meta.Language
	if language == "" {
		language = "en"
	}

	m := &model.Media{
		ID:           mediaID,
		Title:        meta.Title,
		OriginalName: "",
		Year:         meta.Year,
		Quality:      meta.Quality,
		FilePath:     filepath.Join(s.config.DownloadDir, "pending-"+hash),
		FileSize:     0,
		Status:       model.StatusDownloading,
		Source:       model.SourceTorrent,
		Language:     language,
	}

	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("validate media model: %w", err)
	}

	// Clean/truncate magnet link to the first & for transmission-remote compatibility
	transmissionURI := magnetURI
	if idx := strings.Index(transmissionURI, "&"); idx != -1 {
		transmissionURI = transmissionURI[:idx]
	}

	// Add to Transmission
	cmd := exec.Command("transmission-remote", "-a", transmissionURI, "-w", s.config.DownloadDir)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to add torrent to transmission: %w", err)
	}

	if err := s.repo.Create(m); err != nil {
		return nil, fmt.Errorf("create media record: %w", err)
	}

	trackCtx, cancelFunc := context.WithCancel(context.Background())

	s.mu.Lock()
	s.active[mediaID] = &activeTorrent{
		media:            m,
		hash:             hash,
		metadataResolved: false,
		cancelFunc:       cancelFunc,
	}
	s.mu.Unlock()

	go s.trackTorrentDownload(trackCtx, mediaID, hash, m)

	return m, nil
}

func (s *torrentService) GetStatus(mediaID string) (*TorrentStatus, error) {
	s.mu.Lock()
	at, exists := s.active[mediaID]
	s.mu.Unlock()

	if !exists {
		m, err := s.repo.FindByID(mediaID)
		if err != nil {
			return nil, fmt.Errorf("lookup media: %w", err)
		}
		if m == nil {
			return nil, fmt.Errorf("torrent not found: %s", mediaID)
		}
		return &TorrentStatus{
			MediaID:        m.ID,
			Title:          m.Title,
			Status:         string(m.Status),
			TotalBytes:     m.FileSize,
			CompletedBytes: m.FileSize,
			ProgressPct:    100.0,
		}, nil
	}

	if at.transmissionID == "" {
		return &TorrentStatus{
			MediaID: at.media.ID,
			Title:   at.media.Title,
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
			MediaID: at.media.ID,
			Title:   at.media.Title,
			Status:  string(at.media.Status),
		}, nil
	}

	peers := getPeerCount(at.transmissionID)

	return &TorrentStatus{
		MediaID:        at.media.ID,
		Title:          at.media.Title,
		Status:         string(at.media.Status),
		TotalBytes:     at.media.FileSize,
		CompletedBytes: int64(float64(at.media.FileSize) * job.ProgressPct / 100.0),
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
		status := TorrentStatus{
			MediaID: at.media.ID,
			Title:   at.media.Title,
			Status:  string(at.media.Status),
		}

		if at.transmissionID != "" {
			if job, found := jobs[at.transmissionID]; found {
				status.ProgressPct = job.ProgressPct
				status.DownloadRate = job.DownloadRate
				status.TotalBytes = at.media.FileSize
				status.CompletedBytes = int64(float64(at.media.FileSize) * job.ProgressPct / 100.0)
				status.Peers = getPeerCount(at.transmissionID)
			}
		}

		result = append(result, status)
	}
	return result
}

func (s *torrentService) CancelTorrent(mediaID string) error {
	s.mu.Lock()
	at, exists := s.active[mediaID]
	if exists {
		delete(s.active, mediaID)
	}
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("no active torrent download for media: %s", mediaID)
	}

	at.cancelFunc()

	if at.transmissionID != "" {
		_ = exec.Command("transmission-remote", "-t", at.transmissionID, "-rad").Run()
	}

	at.media.Status = model.StatusError
	_ = s.repo.Update(at.media)

	slog.Info("Torrent download cancelled and cleaned up", "mediaID", mediaID, "title", at.media.Title)
	return nil
}

func (s *torrentService) trackTorrentDownload(ctx context.Context, mediaID string, hash string, m *model.Media) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			at, exists := s.active[mediaID]
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
						finalPath := filepath.Join(s.config.DownloadDir, largestFile.Name)

						m.Title = meta.Title
						m.OriginalName = filename
						m.Year = meta.Year
						m.Quality = meta.Quality
						m.Language = meta.Language
						if m.Language == "" {
							m.Language = "en"
						}
						m.FilePath = finalPath
						m.FileSize = largestFile.Size
						_ = s.repo.Update(m)

						s.mu.Lock()
						at.metadataResolved = true
						s.mu.Unlock()
						slog.Info("Torrent metadata resolved in background", "title", m.Title, "file", filename)
					}
				}
			}

			// 3. Track progress and finish download
			if at.metadataResolved {
				jobs, err := s.queryTransmissionList()
				if err == nil {
					job, found := jobs[at.transmissionID]
					if found {
						if job.ProgressPct >= 100.0 {
							slog.Info("Torrent download complete, moving file", "title", m.Title)

							destPath := getUniqueFilePath(s.config.MediaDir, m.OriginalName)
							err := moveFile(m.FilePath, destPath)
							if err != nil {
								slog.Error("Failed to move completed torrent file", "err", err)
								m.Status = model.StatusError
								_ = s.repo.Update(m)
								s.removeActive(mediaID)
								return
							}

							m.FilePath = destPath
							m.Status = model.StatusProcessing
							_ = s.repo.Update(m)

							// Clean up Transmission job and temporary files
							_ = exec.Command("transmission-remote", "-t", at.transmissionID, "-rad").Run()

							s.removeActive(mediaID)
							slog.Info("Torrent download complete, triggered background processing", "title", m.Title)

							// Trigger background processing (ffprobe, main thumbnail, scrubber thumbnails)
							ProcessMediaBackground(s.config, s.repo, m.ID, destPath)
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
	downloading, err := s.repo.FindByStatus(model.StatusDownloading)
	if err != nil {
		slog.Error("Failed to lookup downloading torrents on startup", "err", err)
		return
	}

	jobs, err := s.queryTransmissionList()
	if err != nil {
		slog.Error("Failed to query transmission list on startup", "err", err)
		return
	}

	for _, mVal := range downloading {
		m := mVal
		var matchedJob *TransmissionJob
		mTitle := strings.ToLower(m.Title)
		mOrigName := strings.ToLower(m.OriginalName)
		for _, job := range jobs {
			jobTitle := strings.ToLower(job.Title)
			if (mTitle != "" && (strings.Contains(jobTitle, mTitle) || strings.Contains(mTitle, jobTitle))) ||
				(mOrigName != "" && (strings.Contains(jobTitle, mOrigName) || strings.Contains(mOrigName, jobTitle))) {
				matchedJob = job
				break
			}
		}

		if matchedJob != nil {
			hash, _ := getTorrentHashFromInfo(matchedJob.ID)
			trackCtx, cancelFunc := context.WithCancel(context.Background())
			at := &activeTorrent{
				media:            &m,
				hash:             hash,
				transmissionID:   matchedJob.ID,
				metadataResolved: m.OriginalName != "",
				cancelFunc:       cancelFunc,
			}
			s.mu.Lock()
			s.active[m.ID] = at
			s.mu.Unlock()

			go s.trackTorrentDownload(trackCtx, m.ID, hash, &m)
			slog.Info("Resumed tracking torrent download on startup", "title", m.Title, "transmissionID", matchedJob.ID)
		} else {
			m.Status = model.StatusError
			_ = s.repo.Update(&m)
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
		if strings.HasSuffix(fields[1], "%") {
			donePct, _ = strconv.ParseFloat(strings.TrimSuffix(fields[1], "%"), 64)
		}
		sizeStr := fields[4] + " " + fields[5]
		size := parseSizeToBytes(sizeStr)
		name := strings.Join(fields[6:], " ")
		files = append(files, TorrentFile{
			Index: idx,
			Done:  donePct,
			Size:  size,
			Name:  name,
		})
	}
	return files, nil
}

func getTorrentHash(magnet string) string {
	u, err := url.Parse(magnet)
	if err != nil {
		re := regexp.MustCompile(`(?i)xt=urn:btih:([a-f0-9]{32,40})`)
		matches := re.FindStringSubmatch(magnet)
		if len(matches) > 1 {
			return strings.ToLower(matches[1])
		}
		return ""
	}
	xt := u.Query().Get("xt")
	if strings.HasPrefix(xt, "urn:btih:") {
		return strings.ToLower(strings.TrimPrefix(xt, "urn:btih:"))
	}
	// Fallback to regex
	re := regexp.MustCompile(`(?i)xt=urn:btih:([a-f0-9]{32,40})`)
	matches := re.FindStringSubmatch(magnet)
	if len(matches) > 1 {
		return strings.ToLower(matches[1])
	}
	return ""
}

func parseSpeedBps(speedStr string) float64 {
	fields := strings.Fields(speedStr)
	if len(fields) == 0 {
		return 0
	}
	val, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	if len(fields) < 2 {
		return val
	}
	unit := strings.ToLower(fields[1])
	if strings.Contains(unit, "mb") {
		return val * 1024 * 1024
	}
	if strings.Contains(unit, "kb") {
		return val * 1024
	}
	if strings.Contains(unit, "gb") {
		return val * 1024 * 1024 * 1024
	}
	return val
}

func parseSizeToBytes(sizeStr string) int64 {
	fields := strings.Fields(sizeStr)
	if len(fields) == 0 {
		return 0
	}
	val, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	if len(fields) < 2 {
		return int64(val)
	}
	unit := strings.ToLower(fields[1])
	if strings.Contains(unit, "gb") {
		return int64(val * 1024 * 1024 * 1024)
	}
	if strings.Contains(unit, "mb") {
		return int64(val * 1024 * 1024)
	}
	if strings.Contains(unit, "kb") {
		return int64(val * 1024)
	}
	return int64(val)
}

func getPeerCount(transmissionID string) int {
	cmd := exec.Command("transmission-remote", "-t", transmissionID, "-i")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	re := regexp.MustCompile(`Peers:\s+connected\s+to\s+(\d+)`)
	matches := re.FindStringSubmatch(string(out))
	if len(matches) > 1 {
		val, _ := strconv.Atoi(matches[1])
		return val
	}
	return 0
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

