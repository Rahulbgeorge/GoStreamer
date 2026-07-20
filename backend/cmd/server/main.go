package main

// @title StreamingPlayer API
// @version 1.0
// @description REST API Engine driving the StreamingPlayer media pipeline.
// @host localhost:8080
// @BasePath /api/v1

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "streamingplayer/docs" // Load generated swagger docs
	"streamingplayer/internal/config"
	"streamingplayer/internal/controller"
	"streamingplayer/internal/repository/sqlite"
	"streamingplayer/internal/service"
)

func main() {
	// 1. Load configs & configure structured slog logger
	cfg := config.Load()
	slog.Info("Config", "cfg", cfg)
	cfg.SetupLogger()

	wd, _ := os.Getwd()
	slog.Info("Starting StreamingPlayer Server...", "working_dir", wd)

	// 2. Prepare folders
	prepareFolders(cfg)

	// 3. Connect to database and execute schema migrations
	db, err := setupDatabase(cfg)
	if err != nil {
		slog.Error("Database connection failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// 4. Initialize Dependency Injections
	mediaRepo := sqlite.NewMediaRepository(db)
	prefRepo := sqlite.NewPreferenceRepository(db)
	downloadRepo := sqlite.NewDownloadRepository(db)

	streamService := service.NewStreamService(mediaRepo)
	scannerService := service.NewScannerService(cfg, mediaRepo, prefRepo)
	uploadService := service.NewUploadService(cfg, mediaRepo, prefRepo)
	torrentService, err := service.NewTorrentService(cfg, mediaRepo, prefRepo, downloadRepo, scannerService)
	if err != nil {
		slog.Error("Failed to initialize torrent service", "err", err)
		os.Exit(1)
	}
	defer torrentService.Close()

	youtubeDownloader := service.NewYoutubeDownloader(cfg, downloadRepo, prefRepo, scannerService)
	youtubeCtrl := controller.NewYoutubeController(cfg, youtubeDownloader, mediaRepo, prefRepo, downloadRepo, scannerService)

	mediaCtrl := controller.NewMediaController(mediaRepo, scannerService)
	streamCtrl := controller.NewStreamController(streamService, cfg)
	uploadCtrl := controller.NewUploadController(uploadService)
	torrentCtrl := controller.NewTorrentController(torrentService)
	prefCtrl := controller.NewPreferenceController(prefRepo)
	systemCtrl := controller.NewSystemController()
	downloadCtrl := controller.NewDownloadController(downloadRepo, torrentService)

	// 5. Start file auto-discovery services
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := scannerService.Start(ctx); err != nil {
		slog.Error("Failed to start scanner service", "err", err)
		os.Exit(1)
	}
	defer scannerService.Stop()

	// 6. Setup Gin Router
	router := setupRouter(cfg, mediaCtrl, streamCtrl, uploadCtrl, torrentCtrl, youtubeCtrl, prefCtrl, systemCtrl, downloadCtrl)

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	// 7. Start server with Graceful Shutdown capabilities
	go func() {
		slog.Info("HTTP Server listening", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP Server failed", "err", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down HTTP Server gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP Server forced to shutdown", "err", err)
	}

	slog.Info("Server exited safely")
}

// prepareFolders creates directories for media, downloads, uploads, and thumbnails.
func prepareFolders(cfg *config.Config) {
	for _, path := range []string{cfg.MediaDir, cfg.DownloadDir, cfg.UploadDir, cfg.ThumbnailDir, cfg.YoutubeDownloadDir} {
		if err := os.MkdirAll(path, 0755); err != nil {
			slog.Error("Failed to initialize system folder", "path", path, "err", err)
			os.Exit(1)
		}
	}
}
