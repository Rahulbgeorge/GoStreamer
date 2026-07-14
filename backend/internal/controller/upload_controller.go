package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"streamingplayer/internal/service"
)

type UploadController struct {
	uploadService service.UploadService
}

func NewUploadController(uploadService service.UploadService) *UploadController {
	return &UploadController{uploadService: uploadService}
}

type InitUploadInput struct {
	Filename  string `json:"filename" binding:"required"`
	TotalSize int64  `json:"total_size" binding:"required"`
}

func (ctrl *UploadController) InitUpload(c *gin.Context) {
	var input InitUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	uploadID, err := ctrl.uploadService.InitUpload(input.Filename, input.TotalSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"upload_id": uploadID}})
}

func (ctrl *UploadController) UploadChunk(c *gin.Context) {
	uploadID := c.Param("id")
	indexStr := c.PostForm("index")
	if indexStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing chunk index"})
		return
	}

	chunkIdx, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chunk index"})
		return
	}

	fileHeader, err := c.FormFile("chunk")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing chunk file"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open chunk file"})
		return
	}
	defer file.Close()

	if err := ctrl.uploadService.StoreChunk(uploadID, chunkIdx, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": true})
}

func (ctrl *UploadController) CompleteUpload(c *gin.Context) {
	uploadID := c.Param("id")

	media, err := ctrl.uploadService.CompleteUpload(c.Request.Context(), uploadID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": media})
}
