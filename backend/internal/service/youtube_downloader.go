package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/kkdai/youtube/v2"
	"streamingplayer/internal/config"
	"streamingplayer/internal/repository"
)

// Downloader interface for external clients to use
type YoutubeDownloader interface {
	Download(downloadOptions DownloadOptions) (string, error)
	ListQuality(videoUrl string) (map[string]int, error)
	ListFormats(videoUrl string) ([]youtube.Format, string, error)
	DownloadAdaptive(ctx context.Context, downloadID string, url string, videoItag, audioItag int, destPath string) error
}

type YoutubeDownloaderImpl struct {
	client         *youtube.Client
	cfg            *config.Config
	downloadRepo   repository.DownloadRepository
	prefRepo       repository.PreferenceRepository
	scannerService ScannerService
}

func NewYoutubeDownloader(
	cfg *config.Config,
	downloadRepo repository.DownloadRepository,
	prefRepo repository.PreferenceRepository,
	scannerService ScannerService,
) *YoutubeDownloaderImpl {
	return &YoutubeDownloaderImpl{
		client:         &youtube.Client{},
		cfg:            cfg,
		downloadRepo:   downloadRepo,
		prefRepo:       prefRepo,
		scannerService: scannerService,
	}
}

type DownloadOptions struct {
	VideoUrl         string
	QualityItag      int
	DownloadLocation string
}

func (y *YoutubeDownloaderImpl) Download(downloadOptions DownloadOptions) (string, error) {
	video, err := y.client.GetVideo(downloadOptions.VideoUrl)
	if err != nil {
		return "", err
	}

	itag := downloadOptions.QualityItag
	if itag == 0 {
		itag = 22
	}

	formats := video.Formats.Itag(itag)
	if len(formats) == 0 {
		formats = video.Formats.Itag(18)
		if len(formats) == 0 && len(video.Formats) > 0 {
			formats = []youtube.Format{video.Formats[0]}
		}
	}

	if len(formats) == 0 {
		return "", errors.New("format not found")
	}
	format := &formats[0]

	stream, _, err := y.client.GetStream(video, format)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	out, err := os.Create(downloadOptions.DownloadLocation)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, stream)
	if err != nil {
		return "", err
	}

	return downloadOptions.DownloadLocation, nil
}

func (y *YoutubeDownloaderImpl) ListQuality(videoUrl string) (map[string]int, error) {
	qualityDict := make(map[string]int)

	video, err := y.client.GetVideo(videoUrl)
	if err != nil {
		return nil, err
	}

	for _, f := range video.Formats {
		qualityDict[f.QualityLabel] = f.ItagNo
	}

	return qualityDict, nil
}

func (y *YoutubeDownloaderImpl) ListFormats(videoUrl string) ([]youtube.Format, string, error) {
	video, err := y.client.GetVideo(videoUrl)
	if err != nil {
		return nil, "", err
	}
	return video.Formats, video.Title, nil
}

type progressTracker struct {
	mu           sync.Mutex
	downloadID   string
	totalBytes   int64
	writtenBytes int64
	lastUpdate   time.Time
	updateFreq   time.Duration
	downloadRepo repository.DownloadRepository
}

func (t *progressTracker) AddProgress(n int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.writtenBytes += n

	now := time.Now()
	if now.Sub(t.lastUpdate) >= t.updateFreq || t.writtenBytes >= t.totalBytes {
		t.lastUpdate = now
		var progress float64
		if t.totalBytes > 0 {
			progress = float64(t.writtenBytes) / float64(t.totalBytes) * 100.0
			if progress > 100.0 {
				progress = 100.0
			}
		}

		dl, err := t.downloadRepo.FindByID(t.downloadID)
		if err == nil && dl != nil {
			dl.Progress = progress
			dl.CompletedSize = t.writtenBytes
			dl.TotalSize = t.totalBytes

			durationSecs := now.Sub(dl.CreatedAt).Seconds()
			if durationSecs > 0 {
				dl.DownloadSpeed = float64(t.writtenBytes) / durationSecs
				remainingBytes := t.totalBytes - t.writtenBytes
				if dl.DownloadSpeed > 0 && remainingBytes > 0 {
					etaSecs := int64(float64(remainingBytes) / dl.DownloadSpeed)
					dl.ETA = fmt.Sprintf("%ds", etaSecs)
				}
			}

			_ = t.downloadRepo.Update(dl)
		}
	}
}

type progressWriter struct {
	writer  io.Writer
	tracker *progressTracker
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if n > 0 {
		pw.tracker.AddProgress(int64(n))
	}
	return n, err
}

func (y *YoutubeDownloaderImpl) DownloadAdaptive(ctx context.Context, downloadID string, url string, videoItag, audioItag int, destPath string) error {
	video, err := y.client.GetVideo(url)
	if err != nil {
		return err
	}

	var videoFormat *youtube.Format
	var audioFormat *youtube.Format

	// 1. Determine Video Format
	if videoItag != 0 {
		formats := video.Formats.Itag(videoItag)
		if len(formats) > 0 {
			videoFormat = &formats[0]
		}
	}

	if videoFormat == nil {
		targets := []int{1080, 720, 480, 360, 240, 144}
		for _, targetHeight := range targets {
			for _, f := range video.Formats {
				if f.Height == targetHeight {
					videoFormat = &f
					break
				}
			}
			if videoFormat != nil {
				break
			}
		}
	}

	if videoFormat == nil {
		for _, f := range video.Formats {
			if f.Height > 0 {
				videoFormat = &f
				break
			}
		}
	}
	if videoFormat == nil && len(video.Formats) > 0 {
		videoFormat = &video.Formats[0]
	}
	if videoFormat == nil {
		return errors.New("no suitable video format found")
	}

	// 2. Determine Audio Format
	if audioItag != 0 {
		formats := video.Formats.Itag(audioItag)
		if len(formats) > 0 {
			audioFormat = &formats[0]
		}
	}

	if audioFormat == nil {
		var bestAudio *youtube.Format
		for i := range video.Formats {
			f := &video.Formats[i]
			if f.Height == 0 && (f.AudioQuality != "" || f.Bitrate > 0) {
				if bestAudio == nil || f.Bitrate > bestAudio.Bitrate {
					bestAudio = f
				}
			}
		}
		audioFormat = bestAudio
	}

	// If no audio-only format is found
	if audioFormat == nil {
		totalBytes := videoFormat.ContentLength
		tracker := &progressTracker{
			downloadID:   downloadID,
			totalBytes:   totalBytes,
			lastUpdate:   time.Now(),
			updateFreq:   time.Duration(y.cfg.DownloadUpdateInterval) * time.Second,
			downloadRepo: y.downloadRepo,
		}

		tempPath := filepath.Join(y.cfg.DownloadDir, fmt.Sprintf("yt_temp_%s.mp4.part", downloadID))
		if err := y.downloadStreamWithTracker(video, videoFormat, tempPath, tracker); err != nil {
			return err
		}

		if err := moveFile(tempPath, destPath); err != nil {
			return err
		}

		return nil
	}

	totalBytes := videoFormat.ContentLength + audioFormat.ContentLength
	tracker := &progressTracker{
		downloadID:   downloadID,
		totalBytes:   totalBytes,
		lastUpdate:   time.Now(),
		updateFreq:   time.Duration(y.cfg.DownloadUpdateInterval) * time.Second,
		downloadRepo: y.downloadRepo,
	}

	videoTemp := filepath.Join(y.cfg.DownloadDir, fmt.Sprintf("video_temp_%s.tmp.part", downloadID))
	audioTemp := filepath.Join(y.cfg.DownloadDir, fmt.Sprintf("audio_temp_%s.tmp.part", downloadID))

	var wg sync.WaitGroup
	wg.Add(2)

	var videoErr, audioErr error

	go func() {
		defer wg.Done()
		videoErr = y.downloadStreamWithTracker(video, videoFormat, videoTemp, tracker)
	}()

	go func() {
		defer wg.Done()
		audioErr = y.downloadStreamWithTracker(video, audioFormat, audioTemp, tracker)
	}()

	wg.Wait()

	if videoErr != nil {
		return fmt.Errorf("video download error: %w", videoErr)
	}
	if audioErr != nil {
		return fmt.Errorf("audio download error: %w", audioErr)
	}

	tempMergePath := destPath + ".part"
	cmd := exec.Command("ffmpeg", "-y", "-i", videoTemp, "-i", audioTemp, "-c", "copy", tempMergePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		cmdFallback := exec.Command("ffmpeg", "-y", "-i", videoTemp, "-i", audioTemp, "-c:v", "copy", "-c:a", "aac", tempMergePath)
		outputFallback, errFallback := cmdFallback.CombinedOutput()
		if errFallback != nil {
			_ = os.Remove(videoTemp)
			_ = os.Remove(audioTemp)
			_ = os.Remove(tempMergePath)
			return fmt.Errorf("ffmpeg merge failed: %v. Output: %s. Fallback Output: %s", err, string(output), string(outputFallback))
		}
	}

	_ = os.Remove(videoTemp)
	_ = os.Remove(audioTemp)

	if err := os.Rename(tempMergePath, destPath); err != nil {
		return fmt.Errorf("failed to finalize merged video: %w", err)
	}

	return nil
}

func (y *YoutubeDownloaderImpl) downloadStreamWithTracker(video *youtube.Video, format *youtube.Format, path string, tracker *progressTracker) error {
	stream, _, err := y.client.GetStream(video, format)
	if err != nil {
		return err
	}
	defer stream.Close()

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	pw := &progressWriter{
		writer:  out,
		tracker: tracker,
	}

	_, err = io.Copy(pw, stream)
	return err
}
