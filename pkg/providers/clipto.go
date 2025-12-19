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

type Clipto struct {
	Client HTTPClient
}

func (p *Clipto) Name() string { return "clipto.com" }

func (p *Clipto) GetLink(ytURL string) (*models.VideoResult, error) {
	reqInit, _ := http.NewRequest("GET", "https://www.clipto.com/ru/media-downloader/youtube-downloader", nil)
	if resp, err := p.Client.Do(reqInit); err == nil {
		cerr := resp.Body.Close()
		if cerr != nil {
			slog.Warn("Failed to close response body", "err", cerr)
		}
	}

	payload := map[string]string{"url": ytURL}
	bodyBytes, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://www.clipto.com/api/youtube", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.clipto.com")
	req.Header.Set("Referer", "https://www.clipto.com/ru/media-downloader/youtube-downloader")

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
		Success bool   `json:"success"`
		Title   string `json:"title"`
		Medias  []struct {
			Extension string `json:"extension"`
			IsAudio   bool   `json:"is_audio"`
			Url       string `json:"url"`
			Height    int    `json:"height"`
			Type      string `json:"type"` // "video" or "audio"
			Quality   string `json:"quality"`
		} `json:"medias"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, errors.New("clipto api returned success: false")
	}

	var (
		muxedUrl     string
		videoOnlyUrl string
		audioUrl     string
		targetRes    = 0
	)

	priorities := []int{480, 360, 720}

	for _, res := range priorities {
		for _, m := range result.Medias {
			if m.Type == "video" && m.Extension == "mp4" && m.IsAudio && m.Height == res {
				muxedUrl = m.Url
				break
			}
		}
		if muxedUrl != "" {
			break
		}
	}

	if muxedUrl != "" {
		return &models.VideoResult{
			Title:       result.Title,
			DownloadURL: muxedUrl,
			NeedsMuxing: false,
			Extension:   "mp4",
		}, nil
	}

	for _, m := range result.Medias {
		if m.Type == "video" && m.Extension == "mp4" {
			if m.Height >= 360 && m.Height <= 1080 {
				if videoOnlyUrl == "" || (m.Height == 480) || (targetRes != 480 && m.Height == 360) {
					videoOnlyUrl = m.Url
					targetRes = m.Height
				}
			}
		}
	}

	for _, m := range result.Medias {
		if m.Type == "audio" && m.Extension == "m4a" {
			audioUrl = m.Url
			break
		}
	}

	if videoOnlyUrl != "" && audioUrl != "" {
		return &models.VideoResult{
			Title:       result.Title,
			DownloadURL: videoOnlyUrl,
			AudioURL:    audioUrl,
			NeedsMuxing: true,
			Extension:   "mp4",
		}, nil
	}

	fmt.Println("DEBUG: Clipto available streams:")
	for _, m := range result.Medias {
		fmt.Printf("- Type:%s Ext:%s H:%d IsAudio:%v\n", m.Type, m.Extension, m.Height, m.IsAudio)
	}

	return nil, errors.New("clipto: suitable streams not found")
}
