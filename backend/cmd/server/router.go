package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"streamingplayer/internal/config"
	"streamingplayer/internal/controller"
	"streamingplayer/internal/middleware"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func setupRouter(
	cfg *config.Config,
	mediaCtrl *controller.MediaController,
	streamCtrl *controller.StreamController,
	uploadCtrl *controller.UploadController,
	torrentCtrl *controller.TorrentController,
	youtubeCtrl *controller.YoutubeController,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(middleware.Logger(), middleware.CORS(), gin.Recovery())

	// Register API router group
	api := router.Group("/api/v1")

	// Media Routes
	api.GET("/media", mediaCtrl.GetAllMedia)
	api.GET("/media/:id", mediaCtrl.GetMediaByID)
	api.PUT("/media/:id", mediaCtrl.UpdateMedia)
	api.DELETE("/media/:id", mediaCtrl.DeleteMedia)
	api.GET("/media/stats", mediaCtrl.GetLibraryStats)
	api.GET("/media/search", mediaCtrl.SearchMedia)
	api.POST("/media/scan", mediaCtrl.ScanMedia)

	// Stream Routes
	api.GET("/stream/:id", streamCtrl.StreamVideo)
	api.GET("/media/:id/thumbnail", streamCtrl.StreamThumbnail)
	api.GET("/media/:id/scrubber", streamCtrl.GetScrubberStatus)
	api.GET("/media/:id/scrubber/image/:frame", streamCtrl.StreamScrubberImage)

	// Upload Routes
	api.POST("/upload/init", uploadCtrl.InitUpload)
	api.POST("/upload/:id/chunk", uploadCtrl.UploadChunk)
	api.POST("/upload/:id/complete", uploadCtrl.CompleteUpload)

	// Torrent Routes
	api.POST("/torrent/download", torrentCtrl.DownloadTorrent)
	api.GET("/torrent/status", torrentCtrl.ListActiveTorrents)
	api.GET("/torrent/status/:id", torrentCtrl.GetTorrentStatus)
	api.POST("/torrent/cancel/:id", torrentCtrl.CancelTorrent)
	api.POST("/torrent/scan-url", torrentCtrl.ScanTorrentURL)

	// Youtube Routes (Root level as requested)
	router.GET("/youtube/list", youtubeCtrl.ListFormats)
	router.GET("/youtube/download", youtubeCtrl.DownloadVideo)

	// API versioned Youtube Routes for consistency
	api.GET("/youtube/list", youtubeCtrl.ListFormats)
	api.GET("/youtube/download", youtubeCtrl.DownloadVideo)

	// Exposed Local IP & Ping endpoints
	api.GET("/local-ip", func(c *gin.Context) {
		ip := getLocalIP()
		port := cfg.ServerPort
		c.JSON(http.StatusOK, gin.H{
			"local_ip":  ip,
			"port":      port,
			"local_url": fmt.Sprintf("http://%s:%s", ip, port),
		})
	})

	api.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	frontendDist := findFrontendDist()

	// Serve React Frontend static production bundle assets
	router.Static("/assets", filepath.Join(frontendDist, "assets"))
	router.StaticFile("/vite.svg", filepath.Join(frontendDist, "vite.svg"))
	router.StaticFile("/favicon.svg", filepath.Join(frontendDist, "favicon.svg"))

	// Register Swagger route UI page
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(frontendDist, "index.html"))
	})
	// SPA Fallback: Serve built index.html for all non-API paths
	router.NoRoute(func(c *gin.Context) {
		c.File(filepath.Join(frontendDist, "index.html"))
	})

	return router
}

// getLocalIP queries the outbound interface IP address using UDP or interface scanning.
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String()
	}

	// Fallback to iterating interfaces
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// findFrontendDist searches for the frontend/dist folder in common relative locations.
func findFrontendDist() string {
	distPaths := []string{
		"../frontend/dist",
		"../../frontend/dist",
		"../../../frontend/dist",
		"frontend/dist",
	}
	var statErrors []string
	for _, path := range distPaths {
		absPath, _ := filepath.Abs(path)
		if fi, err := os.Stat(path); err == nil && fi.IsDir() {
			return path
		} else if err != nil {
			statErrors = append(statErrors, fmt.Sprintf("%s (%s): %v", path, absPath, err))
		} else {
			statErrors = append(statErrors, fmt.Sprintf("%s (%s): not a directory", path, absPath))
		}
	}
	slog.Warn("Frontend dist directory not found, defaulting to fallback path", "attempts", statErrors)
	return "../../frontend/dist"
}
