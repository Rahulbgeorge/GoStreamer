package controller

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/gin-gonic/gin"
)

type SystemController struct{}

// NewSystemController instantiates a new controller to browse host directories.
func NewSystemController() *SystemController {
	return &SystemController{}
}

type DirectoryItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (ctrl *SystemController) BrowseDirectory(c *gin.Context) {
	path := c.Query("path")

	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home
		} else {
			// fallback to current working directory
			wd, err := os.Getwd()
			if err == nil {
				path = wd
			} else {
				path = "/"
			}
		}
	}

	// Clean path to be absolute and evaluate any symlinks
	absPath, err := filepath.Abs(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid path format"})
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read directory: " + err.Error()})
		return
	}

	var directories []DirectoryItem
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, DirectoryItem{
				Name: entry.Name(),
				Path: filepath.Join(absPath, entry.Name()),
			})
		}
	}

	// Sort directories alphabetically case-insensitive
	sort.Slice(directories, func(i, j int) bool {
		return filepath.Base(directories[i].Name) < filepath.Base(directories[j].Name)
	})

	parentPath := filepath.Dir(absPath)
	// If already at root, parentPath should be the same as absPath or empty
	if parentPath == absPath {
		parentPath = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"current_path": absPath,
			"parent_path":  parentPath,
			"directories":  directories,
		},
	})
}
