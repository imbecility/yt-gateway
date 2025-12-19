package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/imbecility/yt-gateway/pkg/models"
)

type TechTube struct {
	Client HTTPClient
}

func (p *TechTube) Name() string { return "techtube.cloud" }

func (p *TechTube) GetLink(ytURL string) (*models.VideoResult, error) {
	payload := map[string]string{
		"url":        ytURL,
		"format":     "mp4",
		"resolution": "480",
	}
	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://v0.techtube.cloud/download", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Origin", "https://clipsaver.ru")
	req.Header.Set("Referer", "https://clipsaver.ru/")
	req.Header.Set("Content-Type", "application/json")

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

	var initResp struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&initResp); err != nil {
		return nil, err
	}

	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)

		statusURL := fmt.Sprintf("https://v0.techtube.cloud/status/%s", initResp.TaskID)
		reqStatus, _ := http.NewRequest("GET", statusURL, nil)
		reqStatus.Header.Set("Origin", "https://clipsaver.ru")

		sResp, err := p.Client.Do(reqStatus)
		if err != nil {
			continue
		}

		var statusRes struct {
			Status   string `json:"status"`
			Filename string `json:"filename"`
		}
		jerr := json.NewDecoder(sResp.Body).Decode(&statusRes)
		if jerr != nil {
			slog.Warn("Failed to decode status response", "err", jerr)
		}
		cerr := sResp.Body.Close()
		if cerr != nil {
			slog.Debug("Failed to close response body", "err", cerr)
		}

		if statusRes.Status == "completed" {
			return &models.VideoResult{
				Title:       statusRes.Filename,
				DownloadURL: fmt.Sprintf("https://v0.techtube.cloud/download/file/%s", initResp.TaskID),
				NeedsMuxing: false,
				Extension:   "mp4",
			}, nil
		}
	}

	return nil, errors.New("timeout polling techtube")
}
