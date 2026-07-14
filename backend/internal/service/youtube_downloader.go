package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kkdai/youtube/v2"
)

// Downloader interface for external clients to use
type YoutubeDownloader interface {
	Download(downloadOptions DownloadOptions) (string, error)
	ListQuality(videoUrl string) (map[string]int, error)
	ListFormats(videoUrl string) ([]youtube.Format, string, error)
	DownloadAdaptive(url string, videoItag, audioItag int, destPath string) error
}

type YoutubeDownloaderImpl struct {
	client *youtube.Client
}

func NewYoutubeDownloader() *YoutubeDownloaderImpl {
	return &YoutubeDownloaderImpl{
		client: &youtube.Client{},
	}
}

type DownloadOptions struct {
	VideoUrl         string
	QualityItag      int
	DownloadLocation string
}

func (y *YoutubeDownloaderImpl) Download(downloadOptions DownloadOptions) (string, error) {
	// code for downloading required quality
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
		// Fallback to 18 (360p) which is widely available
		formats = video.Formats.Itag(18)
		if len(formats) == 0 && len(video.Formats) > 0 {
			// Fallback to the first available format
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

func (y *YoutubeDownloaderImpl) DownloadAdaptive(url string, videoItag, audioItag int, destPath string) error {
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
		// Fallback chain: 1080p, 720p, 480p, 360p, 240p, 144p
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

	// If still nil, pick the first video format (or first overall)
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
		// Find best audio-only format based on highest Bitrate
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

	// If no audio-only format is found, check if videoFormat already has audio
	if audioFormat == nil {
		return y.downloadStream(video, videoFormat, destPath)
	}

	// Create a temp directory for separate files
	tmpDir, err := os.MkdirTemp("", "yt_adaptive_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	videoTemp := filepath.Join(tmpDir, "video.tmp")
	audioTemp := filepath.Join(tmpDir, "audio.tmp")

	// Download video stream
	if err := y.downloadStream(video, videoFormat, videoTemp); err != nil {
		return err
	}

	// Download audio stream
	if err := y.downloadStream(video, audioFormat, audioTemp); err != nil {
		return err
	}

	// Merge video and audio with ffmpeg
	cmd := exec.Command("ffmpeg", "-y", "-i", videoTemp, "-i", audioTemp, "-c", "copy", destPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback: copy video and transcode audio to aac
		cmdFallback := exec.Command("ffmpeg", "-y", "-i", videoTemp, "-i", audioTemp, "-c:v", "copy", "-c:a", "aac", destPath)
		outputFallback, errFallback := cmdFallback.CombinedOutput()
		if errFallback != nil {
			return fmt.Errorf("ffmpeg merge failed: %v. Output: %s. Fallback Output: %s", err, string(output), string(outputFallback))
		}
	}

	return nil
}

func (y *YoutubeDownloaderImpl) downloadStream(video *youtube.Video, format *youtube.Format, path string) error {
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

	_, err = io.Copy(out, stream)
	return err
}

