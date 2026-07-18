package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"streamingplayer/internal/model"
	"streamingplayer/internal/repository"
)

type PreferenceController struct {
	repo repository.PreferenceRepository
}

// NewPreferenceController instantiates a controller for settings preference management APIs.
func NewPreferenceController(repo repository.PreferenceRepository) *PreferenceController {
	return &PreferenceController{repo: repo}
}

func (ctrl *PreferenceController) GetAllPreferences(c *gin.Context) {
	list, err := ctrl.repo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch preferences"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func (ctrl *PreferenceController) GetPreference(c *gin.Context) {
	key := c.Param("key")
	pref, err := ctrl.repo.Get(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch preference"})
		return
	}
	if pref == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Preference not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": pref})
}

func (ctrl *PreferenceController) SetPreference(c *gin.Context) {
	var input model.Preference
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request fields"})
		return
	}

	if err := input.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ctrl.repo.Set(input.Key, input.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save preference"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": input})
}

func (ctrl *PreferenceController) DeletePreference(c *gin.Context) {
	key := c.Param("key")
	if err := ctrl.repo.Delete(key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete preference"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": true})
}
