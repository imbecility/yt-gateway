package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/imbecility/yt-gateway/pkg/models"
)

type GetSave struct {
	Client HTTPClient
}

func (p *GetSave) Name() string { return "get-save.com" }

func (p *GetSave) GetLink(ytURL string) (*models.VideoResult, error) {
	payload := map[string]string{"url": ytURL}
	bodyBytes, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://api.get-save.com/api/v1/vidinfo", bytes.NewBuffer(bodyBytes))
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

	var result struct {
		Meta struct {
			Title string `json:"title"`
		} `json:"meta"`
		Sizes []struct {
			Ext        string `json:"ext"`
			Resolution string `json:"resolution"`
			Url        string `json:"url"`
			Height     *int   `json:"height"`
			Acodec     string `json:"acodec"`
		} `json:"sizes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var (
		bestVideoUrl string
		bestAudioUrl string
	)

	priorities := []int{480, 360, 720}
	for _, res := range priorities {
		for _, s := range result.Sizes {
			if s.Height != nil && *s.Height == res && s.Acodec != "none" && s.Ext == "mp4" {
				return &models.VideoResult{
					Title:       result.Meta.Title,
					DownloadURL: s.Url,
					NeedsMuxing: false,
					Extension:   "mp4",
				}, nil
			}
		}
	}

	for _, s := range result.Sizes {
		if s.Ext == "mp4" && s.Height != nil {
			h := *s.Height
			if h >= 360 && h <= 1080 {
				if bestVideoUrl == "" || h == 480 {
					bestVideoUrl = s.Url
				}
			}
		}
	}

	for _, s := range result.Sizes {
		if s.Resolution == "audio only" {
			if s.Ext == "m4a" {
				bestAudioUrl = s.Url
				break
			}
			if bestAudioUrl == "" {
				bestAudioUrl = s.Url
			}
		}
	}

	if bestVideoUrl != "" && bestAudioUrl != "" {
		return &models.VideoResult{
			Title:       result.Meta.Title,
			DownloadURL: bestVideoUrl,
			AudioURL:    bestAudioUrl,
			NeedsMuxing: true,
			Extension:   "mp4",
		}, nil
	}

	fmt.Println("DEBUG: GetSave available sizes:")
	for _, s := range result.Sizes {
		h := -1
		if s.Height != nil {
			h = *s.Height
		}
		fmt.Printf("- Ext:%s Res:%s H:%d Acodec:%s\n", s.Ext, s.Resolution, h, s.Acodec)
	}

	return nil, errors.New("no suitable streams found")
}
