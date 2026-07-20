package controller

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
	"streamingplayer/internal/service"
)

type MediaController struct {
	repo           repository.MediaRepository
	scannerService service.ScannerService
}

func NewMediaController(repo repository.MediaRepository, scannerService service.ScannerService) *MediaController {
	return &MediaController{
		repo:           repo,
		scannerService: scannerService,
	}
}

// GetAllMedia godoc
// @Summary List all library media
// @Description Retrieve a paginated list of cataloged video items.
// @Tags media
// @Produce json
// @Param limit query int false "Items limit" default(20)
// @Param offset query int false "Pagination offset" default(0)
// @Success 200 {object} map[string]interface{} "List of media"
// @Router /media [get]
func (ctrl *MediaController) GetAllMedia(c *gin.Context) {
	limit := getQueryInt(c, "limit", 20)
	offset := getQueryInt(c, "offset", 0)

	list, err := ctrl.repo.FindAll(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch library content"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": list})
}

// GetMediaByID godoc
// @Summary Retrieve single media item details
// @Description Find metadata of a catalog item by its ID.
// @Tags media
// @Produce json
// @Param id path string true "Media ID"
// @Success 200 {object} map[string]interface{} "Media details"
// @Failure 404 {object} map[string]interface{} "Media not found"
// @Router /media/{id} [get]
func (ctrl *MediaController) GetMediaByID(c *gin.Context) {
	id := c.Param("id")
	m, err := ctrl.repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve media record"})
		return
	}
	if m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Movie not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": m})
}

type UpdateMediaInput struct {
	Title    string `json:"title" binding:"required"`
	Year     int    `json:"year"`
	Quality  string `json:"quality"`
	Genre    string `json:"genre"`
	Language string `json:"language"`
}

func (ctrl *MediaController) UpdateMedia(c *gin.Context) {
	id := c.Param("id")
	var input UpdateMediaInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request fields"})
		return
	}

	m, err := ctrl.repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve media record"})
		return
	}
	if m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Movie not found"})
		return
	}

	// Apply updates
	m.Title = input.Title
	if input.Year != 0 {
		m.Year = input.Year
	}
	if input.Quality != "" {
		m.Quality = input.Quality
	}
	if input.Genre != "" {
		m.Genre = input.Genre
	}
	if input.Language != "" {
		m.Language = input.Language
	}

	if err := m.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ctrl.repo.Update(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save media metadata updates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": m})
}

func (ctrl *MediaController) DeleteMedia(c *gin.Context) {
	id := c.Param("id")
	m, err := ctrl.repo.FindByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query media for deletion"})
		return
	}
	if m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Movie not found"})
		return
	}

	// Delete from database
	if err := ctrl.repo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database delete failed"})
		return
	}

	// Optionally try to delete actual files if located in dynamic workspace folders
	_ = os.Remove(m.FilePath)
	service.CleanUpThumbnails(m.ID, m.FilePath)

	c.JSON(http.StatusOK, gin.H{"data": true})
}

type ProgressRequest struct {
	LastPosition int `json:"last_position"`
	Position     int `json:"position"`
}

func (ctrl *MediaController) UpdateProgress(c *gin.Context) {
	id := c.Param("id")
	var req ProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json body"})
		return
	}

	pos := req.LastPosition
	if pos == 0 && req.Position > 0 {
		pos = req.Position
	}

	if err := ctrl.repo.UpdateProgress(id, pos); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "id": id, "last_position": pos})
}

func (ctrl *MediaController) GetLibraryStats(c *gin.Context) {
	count, err := ctrl.repo.Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query media count"})
		return
	}

	totalSize, err := ctrl.repo.TotalSize()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query media size sum"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"count":      count,
		"total_size": totalSize,
	}})
}

func (ctrl *MediaController) SearchMedia(c *gin.Context) {
	q := c.Query("q")
	limit := getQueryInt(c, "limit", 20)
	offset := getQueryInt(c, "offset", 0)

	if q == "" {
		c.JSON(http.StatusOK, gin.H{"data": []model.Media{}})
		return
	}

	list, err := ctrl.repo.Search(q, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": list})
}

func getQueryInt(c *gin.Context, key string, defaultValue int) int {
	valStr := c.Query(key)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue
	}
	return val
}

// ScanMedia godoc
// @Summary Manually trigger library folder scan
// @Description Scan configured media directory for new files and clean up missing ones.
// @Tags media
// @Produce json
// @Success 200 {object} map[string]interface{} "Scan triggered success"
// @Router /media/scan [post]
func (ctrl *MediaController) ScanMedia(c *gin.Context) {
	go ctrl.scannerService.ScanDirectory(context.Background())
	c.JSON(http.StatusOK, gin.H{"data": true, "message": "Folder scanning initiated"})
}
