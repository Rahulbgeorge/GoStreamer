package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds the application configuration parameters.
type Config struct {
	ServerPort        string
	DatabasePath      string
	MediaDir          string
	DownloadDir       string
	UploadDir         string
	ThumbnailDir      string
	LogLevel          string
	DefaultLanguage   string
	AllowedExtensions []string
}

// Load reads config from environment variables or applies defaults.
func Load() *Config {
	homeDir, err := os.UserHomeDir()

	// If running under sudo, override the home directory to target the original user instead of /var/root
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && sudoUser != "root" {
		homeDir = "/Users/" + sudoUser
		err = nil
	}

	var baseDir string
	if err == nil {
		baseDir = filepath.Join(homeDir, ".local", "share", "streamingplayer")
	} else {
		baseDir = "."
	}

	allowedExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".ts"}

	return &Config{
		// ServerPort:        getEnv("SERVER_PORT", "8000"),
		ServerPort:        "8000",
		DatabasePath:      getEnv("DATABASE_PATH", filepath.Join(baseDir, "data", "streaming.db")),
		MediaDir:          getEnv("MEDIA_DIR", filepath.Join(baseDir, "media")),
		DownloadDir:       getEnv("DOWNLOAD_DIR", filepath.Join(baseDir, "downloads")),
		UploadDir:         getEnv("UPLOAD_DIR", filepath.Join(baseDir, "uploads")),
		ThumbnailDir:      getEnv("THUMBNAIL_DIR", filepath.Join(baseDir, "data", "thumbnails")),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		DefaultLanguage:   getEnv("DEFAULT_LANGUAGE", "en"),
		AllowedExtensions: allowedExts,
	}
}

// SetupLogger configures the structured slog handler based on Config settings.
func (c *Config) SetupLogger() {
	var level slog.Level
	switch c.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// In development / local runs we can keep it as clean text.
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
