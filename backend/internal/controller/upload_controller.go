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
	return &UploadController{
		uploadService: uploadService,
	}
}

type InitUploadInput struct {
	Filename    string `json:"filename" binding:"required"`
	TotalSize   int64  `json:"total_size" binding:"required"`
	Fingerprint string `json:"fingerprint"`
	Device      string `json:"device"`
}

func (ctrl *UploadController) CheckUpload(c *gin.Context) {
	fingerprint := c.Query("fingerprint")
	if fingerprint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing fingerprint parameter"})
		return
	}

	uploadID, uploadedChunks, exists, err := ctrl.uploadService.CheckUpload(fingerprint)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"exists":          exists,
			"upload_id":       uploadID,
			"uploaded_chunks": uploadedChunks,
			"chunk_size":      5242880, // 5MB
		},
	})
}

func (ctrl *UploadController) InitUpload(c *gin.Context) {
	var input InitUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	device := input.Device
	if device == "" {
		device = c.Request.UserAgent()
	}

	uploadID, err := ctrl.uploadService.InitUpload(input.Filename, input.TotalSize, input.Fingerprint, device)
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
