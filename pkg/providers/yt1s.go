package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/imbecility/yt-gateway/pkg/models"
	"github.com/imbecility/yt-gateway/pkg/utils"
)

type YT1S struct {
	Client HTTPClient
}

func (p *YT1S) Name() string { return "yt1s.com.co" }

func (p *YT1S) GetLink(ytURL string) (*models.VideoResult, error) {
	vidID := utils.ExtractVideoID(ytURL)
	if vidID == "" {
		return nil, errors.New("invalid youtube url")
	}

	payload := map[string]string{
		"videoId": vidID,
		"quality": "480",
	}
	bodyBytes, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://dlsrv.online/api/download/mp4", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://yt1s.com.co")
	req.Header.Set("Referer", "https://yt1s.com.co/")

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		cerr := Body.Close()
		if cerr != nil {
			slog.Warn("Failed to close response body", "err", cerr)
		}
	}(resp.Body)

	var result struct {
		Success   bool   `json:"success"`
		ModalHtml string `json:"modalHtml"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, errors.New("provider returned success: false")
	}

	re := regexp.MustCompile(`window\.location\.href='([^']+)`)
	matches := re.FindStringSubmatch(result.ModalHtml)
	if len(matches) < 2 {
		return nil, errors.New("download url not found in regex")
	}

	return &models.VideoResult{
		Title:       "YouTube Video (yt1s)",
		DownloadURL: matches[1],
		NeedsMuxing: false,
		Extension:   "mp4",
	}, nil
}
