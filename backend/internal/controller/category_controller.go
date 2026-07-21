package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"

	"github.com/gin-gonic/gin"
)

type CategoryController struct {
	repo repository.CategoryRepository
}

func NewCategoryController(repo repository.CategoryRepository) *CategoryController {
	return &CategoryController{repo: repo}
}

// GetAllCategories handles GET /api/v1/categories
func (ctrl *CategoryController) GetAllCategories(c *gin.Context) {
	categories, err := ctrl.repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if categories == nil {
		categories = []model.Category{}
	}
	c.JSON(http.StatusOK, categories)
}

type createCategoryRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateCategory handles POST /api/v1/categories
func (ctrl *CategoryController) CreateCategory(c *gin.Context) {
	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category name cannot be empty"})
		return
	}

	catID := fmt.Sprintf("cat_%d", time.Now().UnixNano())
	cat := &model.Category{
		ID:   catID,
		Name: name,
	}

	if err := ctrl.repo.Create(cat); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cat)
}

// DeleteCategory handles DELETE /api/v1/categories/:id
func (ctrl *CategoryController) DeleteCategory(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category id required"})
		return
	}

	if err := ctrl.repo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "category deleted successfully"})
}
