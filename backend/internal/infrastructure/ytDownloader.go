package infrastructure

import (
	"errors"
	"io"
	"os"

	"github.com/kkdai/youtube/v2"
)

// Downloader interface for external clients to use
type YoutubeDownloader interface {
	Download(downloadOptions DownloadOptions) (string, error)
	ListQuality(videoUrl string) (map[string]int, error)
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
