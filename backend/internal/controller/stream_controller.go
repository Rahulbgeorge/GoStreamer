package controller

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"streamingplayer/internal/config"
	"streamingplayer/internal/service"
)

type StreamController struct {
	streamService service.StreamService
	cfg           *config.Config
}

func NewStreamController(streamService service.StreamService, cfg *config.Config) *StreamController {
	return &StreamController{streamService: streamService, cfg: cfg}
}

func (ctrl *StreamController) StreamVideo(c *gin.Context) {
	id := c.Param("id")

	file, media, err := ctrl.streamService.GetVideoStream(c.Request.Context(), id)
	if err != nil {
		slog.Error("Stream resolution failed", "id", id, "err", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()

	// Capture modification time for HTTP Cache-Control verification
	stat, err := file.Stat()
	var modTime time.Time
	var fileSize int64
	if err == nil {
		modTime = stat.ModTime()
		fileSize = stat.Size()
	} else {
		modTime = time.Now()
	}

	// Limit range chunk response to 5MB max to protect bandwidth and avoid massive data buffering.
	rangeHeader := c.Request.Header.Get("Range")
	if rangeHeader != "" && strings.HasPrefix(rangeHeader, "bytes=") {
		trimmedRange := strings.TrimPrefix(rangeHeader, "bytes=")
		parts := strings.Split(trimmedRange, "-")
		if len(parts) == 2 {
			start, errStart := strconv.ParseInt(parts[0], 10, 64)
			if errStart == nil && start >= 0 {
				var end int64 = -1
				if parts[1] != "" {
					if parsedEnd, errEnd := strconv.ParseInt(parts[1], 10, 64); errEnd == nil {
						end = parsedEnd
					}
				}

				maxChunk := int64(5 * 1024 * 1024) // 5 MB max chunk limit
				if end == -1 || (end - start + 1) > maxChunk {
					end = start + maxChunk - 1
				}
				if end >= fileSize {
					end = fileSize - 1
				}
				if start < fileSize && start <= end {
					c.Request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
				}
			}
		}
	}

	// ServeContent automatically parses Range request headers and handles 206 Partial Content response
	slog.Debug("Serving video via range request", "title", media.Title, "range", c.Request.Header.Get("Range"))
	
	// Set correct content type
	c.Header("Content-Type", media.MimeType)
	
	http.ServeContent(c.Writer, c.Request, media.OriginalName, modTime, file)
}

func (ctrl *StreamController) StreamThumbnail(c *gin.Context) {
	id := c.Param("id")

	file, err := ctrl.streamService.GetThumbnailStream(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	var modTime time.Time
	if err == nil {
		modTime = stat.ModTime()
	} else {
		modTime = time.Now()
	}

	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "public, max-age=31536000") // Cache thumbnails for a year
	http.ServeContent(c.Writer, c.Request, fmt.Sprintf("%s.jpg", id), modTime, file)
}

func (ctrl *StreamController) GetScrubberStatus(c *gin.Context) {
	id := c.Param("id")

	count, err := ctrl.streamService.GetScrubberStatus(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"ready":    false,
				"interval": 10,
				"count":    0,
			},
		})
		return
	}

	if count == 0 {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"ready":    false,
				"interval": 10,
				"count":    0,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"ready":    true,
			"interval": 10,
			"count":    count,
		},
	})
}

func (ctrl *StreamController) StreamScrubberImage(c *gin.Context) {
	id := c.Param("id")
	frameStr := c.Param("frame")
	frame, err := strconv.Atoi(frameStr)
	if err != nil || frame <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid frame index"})
		return
	}

	file, err := ctrl.streamService.GetScrubberImage(c.Request.Context(), id, frame)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Scrubber frame not found"})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	var modTime time.Time
	if err == nil {
		modTime = stat.ModTime()
	} else {
		modTime = time.Now()
	}

	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "public, max-age=31536000") // Cache scrubber images for a year
	http.ServeContent(c.Writer, c.Request, fmt.Sprintf("scrub_%s_%d.jpg", id, frame), modTime, file)
}

