package thumbnail

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Generate extracts a single thumbnail frame from a video file using ffmpeg.
// It will attempt to seek to 60 seconds into the video.
func Generate(ctx context.Context, videoPath string, outDir string, mediaID string) (string, error) {
	outPath := filepath.Join(outDir, fmt.Sprintf("%s.jpg", mediaID))

	// Locate ffmpeg
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("ffmpeg executable not found in PATH: %w", err)
	}

	// -ss 00:01:00 specifies seeking 1 minute into the video to avoid initial black frames.
	// -vframes 1 instructs ffmpeg to extract exactly 1 frame.
	// -q:v 2 configures high quality JPEG output.
	// -vf scale=480:-1 restricts width to 480px, maintaining aspect ratio.
	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, ffmpegPath,
		"-y", // Overwrite output files without asking
		"-ss", "00:00:20", // Try 20s to ensure shorter videos are also covered safely
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		"-vf", "scale=480:-1",
		outPath,
	)

	slog.Debug("Running ffmpeg command for thumbnail generation", "args", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fallback: try seeking to start if 20 seconds failed (e.g. video is very short)
		slog.Warn("Failed to extract thumbnail at 20s. Retrying at 0s...", "err", err, "output", string(out))
		
		cmdRetryCtx, cancelRetry := context.WithTimeout(ctx, 20*time.Second)
		defer cancelRetry()

		cmdRetry := exec.CommandContext(cmdRetryCtx, ffmpegPath,
			"-y",
			"-i", videoPath,
			"-vframes", "1",
			"-q:v", "2",
			"-vf", "scale=480:-1",
			outPath,
		)
		if retryOut, retryErr := cmdRetry.CombinedOutput(); retryErr != nil {
			return "", fmt.Errorf("ffmpeg thumbnail extractor failed: %v (details: %s)", retryErr, string(retryOut))
		}
	}

	slog.Info("Successfully generated thumbnail", "mediaID", mediaID, "path", outPath)
	return outPath, nil
}

// ProbeDuration extracts video duration (in seconds) and MIME type using ffprobe.
func ProbeDuration(ctx context.Context, videoPath string) (int, string, error) {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, "video/mp4", nil // Fallback silently if ffprobe isn't installed
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Extract duration in seconds
	cmd := exec.CommandContext(cmdCtx, ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, "video/mp4", fmt.Errorf("ffprobe duration query failed: %w (details: %s)", err, string(out))
	}

	var duration float64
	_, err = fmt.Sscanf(string(out), "%f", &duration)
	if err != nil {
		slog.Warn("Failed to parse duration string from ffprobe", "output", string(out), "err", err)
		return 0, "video/mp4", nil
	}

	// Detect MIME type based on extension
	mimeType := "video/mp4"
	ext := strings.ToLower(filepath.Ext(videoPath))
	switch ext {
	case ".mkv":
		mimeType = "video/x-matroska"
	case ".avi":
		mimeType = "video/x-msvideo"
	case ".webm":
		mimeType = "video/webm"
	case ".mov":
		mimeType = "video/quicktime"
	}

	return int(duration), mimeType, nil
}

// GenerateScrubberThumbnails extracts frames from the video at regular intervals (e.g. every 10 seconds).
func GenerateScrubberThumbnails(ctx context.Context, videoPath string, outDir string, mediaID string, intervalSecs int) (int, error) {
	// Locate ffmpeg
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return 0, fmt.Errorf("ffmpeg executable not found in PATH: %w", err)
	}

	// Output pattern: scrub_<mediaID>_%d.jpg
	outPattern := filepath.Join(outDir, fmt.Sprintf("scrub_%s_%%d.jpg", mediaID))

	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute) // Give it up to 5 mins
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, ffmpegPath,
		"-y", // Overwrite output files without asking
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=1/%d,scale=160:-1", intervalSecs),
		"-q:v", "4", // reasonable quality, keep file size small
		outPattern,
	)

	slog.Info("Running ffmpeg for scrubber thumbnails", "args", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return 0, fmt.Errorf("ffmpeg scrubber extractor failed: %v (details: %s)", err, string(out))
	}

	// Let's count how many files were generated
	// They will be scrub_<mediaID>_1.jpg, scrub_<mediaID>_2.jpg, ...
	count := 0
	for {
		filePath := filepath.Join(outDir, fmt.Sprintf("scrub_%s_%d.jpg", mediaID, count+1))
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		count++
	}

	slog.Info("Successfully generated scrubber thumbnails", "mediaID", mediaID, "count", count)
	return count, nil
}

// ProbeDimensions extracts the width and height of a video file using ffprobe.
func ProbeDimensions(ctx context.Context, videoPath string) (int, int, error) {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe executable not found in PATH: %w", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		videoPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe query failed: %w (details: %s)", err, string(out))
	}

	var w, h int
	_, err = fmt.Sscanf(strings.TrimSpace(string(out)), "%dx%d", &w, &h)
	if err != nil {
		return 0, 0, fmt.Errorf("parse dimensions from ffprobe output failed: %w", err)
	}

	return w, h, nil
}

// GenerateAtTime extracts a single frame thumbnail at a specific timestamp (in seconds) from a video file.
func GenerateAtTime(ctx context.Context, videoPath string, outPath string, timestampSecs float64) (string, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("ffmpeg executable not found in PATH: %w", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	timeStr := fmt.Sprintf("%.3f", timestampSecs)

	cmd := exec.CommandContext(cmdCtx, ffmpegPath,
		"-y",
		"-ss", timeStr,
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		"-vf", "scale=480:-1",
		outPath,
	)

	slog.Debug("Extracting thumbnail frame at timestamp", "timestamp", timeStr, "videoPath", videoPath, "outPath", outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg frame extraction failed at %ss: %w (details: %s)", timeStr, err, string(out))
	}

	slog.Info("Successfully extracted frame thumbnail", "timestamp", timeStr, "outPath", outPath)
	return outPath, nil
}



