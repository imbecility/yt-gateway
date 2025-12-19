package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/imbecility/yt-gateway/pkg/api"
	"github.com/imbecility/yt-gateway/pkg/gateway"
)

func main() {
	urlFlag := flag.String("url", "", "YouTube URL or ID")
	ffmpegPath := flag.String("ffmpeg", "ffmpeg", "Path to ffmpeg binary")
	outDir := flag.String("out", "./downloads", "Output directory")
	timeoutFlag := flag.Int("timeout", 60, "Max seconds to wait per attempt")
	debugFlag := flag.Bool("debug", false, "Enable debug logging")

	apiMode := flag.Bool("api", false, "Run in API Server mode")
	apiPort := flag.Int("port", 8080, "Port for API server")
	webMode := flag.Bool("onweb", false, "Enable simple Web UI")
	dlProgress := flag.Bool("dl-progress", false, "Show console progress bar")

	flag.Parse()

	gw, err := gateway.New(gateway.Config{
		OutputDir:    *outDir,
		FFmpegPath:   *ffmpegPath,
		TimeoutSec:   *timeoutFlag,
		Debug:        *debugFlag,
		ShowProgress: *dlProgress,
	})

	if err != nil {
		fmt.Printf("Initialization failed: %v\n", err)
		os.Exit(1)
	}

	// API Server
	if *apiMode {
		srv := &api.Server{
			Port:       *apiPort,
			Gateway:    gw,
			Downloader: gw.Downloader,
			Host:       fmt.Sprintf("http://localhost:%d", *apiPort),
		}

		go srv.BackgroundCleaner(10 * time.Minute)

		if sterr := srv.Start(*webMode); sterr != nil {
			slog.Error("Server crashed", "err", sterr)
			os.Exit(1)
		}
		return
	}

	// CLI
	if *urlFlag == "" {
		slog.Error("Usage: -url <LINK> or -api")
		os.Exit(1)
	}

	slog.Info("Processing video via CLI", "url", *urlFlag)

	res, path, err := gw.ProcessVideo(*urlFlag)
	if err != nil {
		slog.Error("Failed to process video", "err", err)
		os.Exit(1)
	}

	slog.Info("Success", "title", res.Title, "path", path)
}
