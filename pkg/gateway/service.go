package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/imbecility/yt-gateway/pkg/downloader"
	"github.com/imbecility/yt-gateway/pkg/models"
	"github.com/imbecility/yt-gateway/pkg/providers"
	"github.com/imbecility/yt-gateway/pkg/utils"
)

type Service struct {
	Providers  []providers.Provider
	Downloader *downloader.Downloader
	Timeout    time.Duration
}

func NewService(dl *downloader.Downloader, provs []providers.Provider, timeoutSec int) *Service {
	if timeoutSec <= 0 {
		timeoutSec = 60
	}
	return &Service{
		Downloader: dl,
		Providers:  provs,
		Timeout:    time.Duration(timeoutSec) * time.Second,
	}
}

func (s *Service) ProcessVideo(rawURL string) (*models.VideoResult, string, error) {
	vidID := utils.ExtractVideoID(rawURL)
	if vidID == "" {
		return nil, "", errors.New("could not extract video ID")
	}

	fullURL := "https://www.youtube.com/watch?v=" + vidID

	result, providerName, err := s.GetLinkWithRetries(fullURL)
	if err != nil {
		return nil, "", err
	}
	result.VideoID = vidID

	if s.needsBetterTitle(result.Title) {
		slog.Debug("Provider returned generic title, fetching metadata...", "old_title", result.Title)
		realTitle, gterr := providers.GetVideoTitle(s.Downloader.Client, vidID)
		if gterr == nil && realTitle != "" {
			slog.Info("Metadata fetched", "title", realTitle)
			result.Title = realTitle
		} else {
			slog.Warn("Failed to fetch metadata", "err", gterr)
			if result.Title == "" {
				result.Title = "video_" + vidID
			}
		}
	}

	slog.Info("Link acquired", "provider", providerName, "needs_muxing", result.NeedsMuxing)

	finalPath, err := s.Downloader.DownloadAndMux(result)
	if err != nil {
		return nil, "", fmt.Errorf("download/mux failed: %w", err)
	}

	absPath, _ := filepath.Abs(finalPath)
	return result, absPath, nil
}

func (s *Service) GetLinkWithRetries(url string) (*models.VideoResult, string, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		slog.Info("Starting race", "attempt", attempt, "url", url)

		ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
		res, name, err := s.raceProviders(ctx, url)
		cancel()

		if err == nil {
			slog.Info("Race attempt successful", "attempt", attempt, "url", url)
			return res, name, nil
		}

		slog.Warn("Race attempt failed", "attempt", attempt, "err", err)
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	return nil, "", fmt.Errorf("all attempts failed: %w", lastErr)
}

func (s *Service) raceProviders(ctx context.Context, url string) (*models.VideoResult, string, error) {
	type raceResult struct {
		res  *models.VideoResult
		name string
		err  error
	}

	resultChan := make(chan raceResult, len(s.Providers))

	for _, p := range s.Providers {
		go func(p providers.Provider) {
			res, err := p.GetLink(url)
			select {
			case <-ctx.Done():
				return
			case resultChan <- raceResult{res: res, name: p.Name(), err: err}:
			}
		}(p)
	}

	var bestFallback *raceResult
	var timeoutCh <-chan time.Time
	responsesCount := 0
	totalProvs := len(s.Providers)

	for {
		select {
		case r := <-resultChan:
			responsesCount++
			if r.err != nil {
				slog.Debug("Provider response", "provider", r.name, "status", "error", "msg", r.err)
			} else {
				slog.Debug("Provider response", "provider", r.name, "status", "success", "mux", r.res.NeedsMuxing)
			}

			if r.err != nil {
				if responsesCount == totalProvs {
					if bestFallback != nil {
						return bestFallback.res, bestFallback.name, nil
					}
					return nil, "", errors.New("all providers failed")
				}
				continue
			}

			if !r.res.NeedsMuxing {
				return r.res, r.name, nil
			}

			if bestFallback == nil {
				bestFallback = &r
				timeoutCh = time.After(2500 * time.Millisecond)
				slog.Info("Candidate found (needs muxing). Waiting for better...", "provider", r.name)
			}

			if responsesCount == totalProvs {
				return bestFallback.res, bestFallback.name, nil
			}

		case <-timeoutCh:
			if bestFallback != nil {
				slog.Info("Timeout waiting for better option. Using fallback.", "provider", bestFallback.name)
				return bestFallback.res, bestFallback.name, nil
			}

		case <-ctx.Done():
			return nil, "", errors.New("global timeout")
		}
	}
}

func (s *Service) needsBetterTitle(title string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	if t == "" {
		return true
	}
	badTitles := []string{
		"youtube video",
		"video playback",
		"download video",
		"yt1s",
		"loader.do",
		"techtube",
		"mp4",
	}
	for _, bad := range badTitles {
		if strings.Contains(t, bad) {
			return true
		}
	}
	return false
}
