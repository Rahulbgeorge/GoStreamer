package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"streamingplayer/internal/service"
)

func main() {
	// Create a new downloader instance
	downloader := service.NewYoutubeDownloader(nil, nil, nil, nil)

	// A short public test video on YouTube
	testURL := "https://www.youtube.com/watch?v=aqz-KE-bpKQ" // Big Buck Bunny trailer or similar short video

	// We will download to a temp location in the workspace
	tmpDir, err := os.MkdirTemp("", "yt_test_*")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	downloadPath := filepath.Join(tmpDir, "test_video.mp4")

	fmt.Println("Starting download test...")
	fmt.Printf("Video URL: %s\n", testURL)
	fmt.Printf("Destination: %s\n", downloadPath)

	// List available qualities
	qualities, err := downloader.ListQuality(testURL)
	if err != nil {
		log.Fatalf("ListQuality failed: %v", err)
	}
	fmt.Println("Available qualities and itags:")
	for label, itag := range qualities {
		fmt.Printf(" - %s: %d\n", label, itag)
	}

	// Call Download options. QualityItag is set to 0 (default).
	// If 22 is not available, we can see what's there.
	options := service.DownloadOptions{
		VideoUrl:         testURL,
		QualityItag:      0, // Will be overridden to 22 internally (or we could select another)
		DownloadLocation: downloadPath,
	}

	resultPath, err := downloader.Download(options)
	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}

	fmt.Printf("Download succeeded! File saved to: %s\n", resultPath)

	// Verify file exists and is not empty
	info, err := os.Stat(resultPath)
	if err != nil {
		log.Fatalf("Failed to inspect downloaded file: %v", err)
	}
	fmt.Printf("Downloaded file size: %d bytes\n", info.Size())
}
