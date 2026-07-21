package controller

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"streamingplayer/internal/config"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
	"streamingplayer/pkg/thumbnail"

	"github.com/gin-gonic/gin"
)

type ClipController struct {
	cfg       *config.Config
	clipRepo  repository.ClipRepository
	mediaRepo repository.MediaRepository
}

func NewClipController(cfg *config.Config, clipRepo repository.ClipRepository, mediaRepo repository.MediaRepository) *ClipController {
	return &ClipController{
		cfg:       cfg,
		clipRepo:  clipRepo,
		mediaRepo: mediaRepo,
	}
}

// GetClips handles GET /api/v1/clips?media_id=...&category_id=...
func (ctrl *ClipController) GetClips(c *gin.Context) {
	mediaID := c.Query("media_id")
	categoryID := c.Query("category_id")

	var clips []model.Clip
	var err error

	if mediaID != "" {
		clips, err = ctrl.clipRepo.FindByMediaID(mediaID)
	} else if categoryID != "" {
		clips, err = ctrl.clipRepo.FindByCategoryID(categoryID)
	} else {
		clips, err = ctrl.clipRepo.FindAll()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if clips == nil {
		clips = []model.Clip{}
	}
	c.JSON(http.StatusOK, clips)
}

// GetClipByID handles GET /api/v1/clips/:id
func (ctrl *ClipController) GetClipByID(c *gin.Context) {
	id := c.Param("id")
	clip, err := ctrl.clipRepo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if clip == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "clip not found"})
		return
	}
	c.JSON(http.StatusOK, clip)
}

type createClipRequest struct {
	MediaID            string   `json:"media_id" binding:"required"`
	Title              string   `json:"title" binding:"required"`
	StartTime          float64  `json:"start_time"`
	EndTime            float64  `json:"end_time" binding:"required"`
	CategoryIDs        []string `json:"category_ids"`
	ThumbnailFrameTime *float64 `json:"thumbnail_frame_time"`
}

// CreateClip handles POST /api/v1/clips
func (ctrl *ClipController) CreateClip(c *gin.Context) {
	var req createClipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "clip title cannot be empty"})
		return
	}

	media, err := ctrl.mediaRepo.FindByID(req.MediaID)
	if err != nil || media == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "media video not found"})
		return
	}

	if req.EndTime <= req.StartTime {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_time must be greater than start_time"})
		return
	}

	clipID := fmt.Sprintf("clip_%d", time.Now().UnixNano())
	thumbPath := ""

	// Determine thumbnail generation frame timestamp (defaults to start_time if not explicitly given)
	frameTime := req.StartTime
	if req.ThumbnailFrameTime != nil {
		frameTime = *req.ThumbnailFrameTime
	}

	// Extract clip thumbnail frame using ffmpeg
	outPath := filepath.Join(ctrl.cfg.ThumbnailDir, fmt.Sprintf("%s.jpg", clipID))
	if genPath, genErr := thumbnail.GenerateAtTime(c.Request.Context(), media.FilePath, outPath, frameTime); genErr == nil {
		thumbPath = genPath
	} else {
		slog.Warn("Failed to extract clip frame thumbnail", "clipID", clipID, "err", genErr)
	}

	clip := &model.Clip{
		ID:            clipID,
		MediaID:       req.MediaID,
		Title:         title,
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
		ThumbnailPath: thumbPath,
		CategoryIDs:   req.CategoryIDs,
	}

	if err := ctrl.clipRepo.Create(clip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, clip)
}

type updateClipRequest struct {
	Title              string   `json:"title"`
	StartTime          float64  `json:"start_time"`
	EndTime            float64  `json:"end_time"`
	CategoryIDs        []string `json:"category_ids"`
	ThumbnailFrameTime *float64 `json:"thumbnail_frame_time"`
}

// UpdateClip handles PUT /api/v1/clips/:id
func (ctrl *ClipController) UpdateClip(c *gin.Context) {
	id := c.Param("id")
	clip, err := ctrl.clipRepo.FindByID(id)
	if err != nil || clip == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "clip not found"})
		return
	}

	var req updateClipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if strings.TrimSpace(req.Title) != "" {
		clip.Title = strings.TrimSpace(req.Title)
	}
	if req.EndTime > req.StartTime {
		clip.StartTime = req.StartTime
		clip.EndTime = req.EndTime
	}
	if req.CategoryIDs != nil {
		clip.CategoryIDs = req.CategoryIDs
	}

	if req.ThumbnailFrameTime != nil {
		media, _ := ctrl.mediaRepo.FindByID(clip.MediaID)
		if media != nil {
			outPath := filepath.Join(ctrl.cfg.ThumbnailDir, fmt.Sprintf("%s.jpg", clip.ID))
			if genPath, genErr := thumbnail.GenerateAtTime(c.Request.Context(), media.FilePath, outPath, *req.ThumbnailFrameTime); genErr == nil {
				clip.ThumbnailPath = genPath
			}
		}
	}

	if err := ctrl.clipRepo.Update(clip); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, clip)
}

// DeleteClip handles DELETE /api/v1/clips/:id
func (ctrl *ClipController) DeleteClip(c *gin.Context) {
	id := c.Param("id")
	clip, _ := ctrl.clipRepo.FindByID(id)
	if clip != nil && clip.ThumbnailPath != "" {
		_ = os.Remove(clip.ThumbnailPath)
	}

	if err := ctrl.clipRepo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "clip deleted successfully"})
}

// StreamClipThumbnail handles GET /api/v1/clips/:id/thumbnail
func (ctrl *ClipController) StreamClipThumbnail(c *gin.Context) {
	id := c.Param("id")
	clip, err := ctrl.clipRepo.FindByID(id)
	if err != nil || clip == nil || clip.ThumbnailPath == "" {
		c.Status(http.StatusNotFound)
		return
	}

	if _, err := os.Stat(clip.ThumbnailPath); os.IsNotExist(err) {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Cache-Control", "public, max-age=86400")
	c.File(clip.ThumbnailPath)
}
