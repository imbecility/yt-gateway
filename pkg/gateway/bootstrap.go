package gateway

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/imbecility/yt-gateway/pkg/client"
	"github.com/imbecility/yt-gateway/pkg/downloader"
	"github.com/imbecility/yt-gateway/pkg/ffmpeg"
	"github.com/imbecility/yt-gateway/pkg/logger"
	"github.com/imbecility/yt-gateway/pkg/providers"
)

// Config represents the configuration for gateway initialization.
type Config struct {
	// OutputDir is the folder for saving files (defaults to ./downloads).
	OutputDir string
	// FFmpegPath is the path to the ffmpeg executable (defaults to "ffmpeg").
	FFmpegPath string
	// TimeoutSec is the timeout for searching for a link in seconds (defaults to 60).
	TimeoutSec int
	// Debug enables verbose logging.
	Debug bool
	// ShowProgress enables the progress bar in the console (for CLI usage).
	ShowProgress bool
}

// New creates a ready-to-use Service instance with all necessary dependencies.
func New(cfg Config) (*Service, error) {
	// Setup the logger (globally)
	logger.SetupGlobal(cfg.Debug, false)

	// Set default values
	if cfg.OutputDir == "" {
		cfg.OutputDir = "./downloads"
	}
	if cfg.FFmpegPath == "" {
		cfg.FFmpegPath = "ffmpeg"
	}
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = 60
	}

	// Create the directory
	absOutDir, err := filepath.Abs(cfg.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("invalid output dir: %w", err)
	}
	if err := os.MkdirAll(absOutDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output dir: %w", err)
	}

	// Initialize the HTTP client
	httpClient, err := client.NewHttpClient()
	if err != nil {
		return nil, fmt.Errorf("failed to init http client: %w", err)
	}

	// Checking and downloading FFmpeg
	realFFmpegPath, err := ffmpeg.EnsureBinary(httpClient, cfg.FFmpegPath)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg check failed: %w", err)
	}
	cfg.FFmpegPath = realFFmpegPath

	// Register providers
	provs := []providers.Provider{
		&providers.YT1S{Client: httpClient},
		&providers.LoaderDo{Client: httpClient},
		&providers.TechTube{Client: httpClient},
		&providers.Clipto{Client: httpClient},
		&providers.GetSave{Client: httpClient},
	}

	// Initialize the downloader
	dl := &downloader.Downloader{
		Client:       httpClient,
		FFmpegPath:   cfg.FFmpegPath,
		OutputDir:    absOutDir,
		ShowProgress: cfg.ShowProgress,
	}

	// Return the service
	return NewService(dl, provs, cfg.TimeoutSec), nil
}
