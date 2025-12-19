package providers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/imbecility/yt-gateway/pkg/models"
)

type LoaderDo struct {
	Client HTTPClient
}

func (p *LoaderDo) Name() string { return "loader.do" }

func (p *LoaderDo) GetLink(ytURL string) (*models.VideoResult, error) {
	apiKey := "dfcb6d76f2f6a9894gjkege8a4ab232222"

	params := url.Values{}
	params.Add("copyright", "0")
	params.Add("format", "480")
	params.Add("url", ytURL)
	params.Add("api", apiKey)

	initUrl := fmt.Sprintf("https://p.savenow.to/ajax/download.php?%s", params.Encode())

	req, _ := http.NewRequest("GET", initUrl, nil)
	req.Header.Set("Origin", "https://loader.do")
	req.Header.Set("Referer", "https://loader.do/")

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Warn("Failed to close response body", "err", err)
		}
	}(resp.Body)

	var initRes struct {
		Success bool   `json:"success"`
		ID      string `json:"id"`
		Title   string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&initRes); err != nil {
		return nil, err
	}
	if !initRes.Success || initRes.ID == "" {
		return nil, errors.New("loader.do init failed")
	}

	progressUrl := fmt.Sprintf("https://p.savenow.to/api/progress?id=%s", initRes.ID)

	for i := 0; i < 20; i++ {
		time.Sleep(2 * time.Second)

		reqP, _ := http.NewRequest("GET", progressUrl, nil)
		reqP.Header.Set("Origin", "https://loader.do")
		reqP.Header.Set("Referer", "https://loader.do/")

		respP, err := p.Client.Do(reqP)
		if err != nil {
			continue
		}

		var progRes struct {
			Success     int    `json:"success"` // 1
			Text        string `json:"text"`    // "Finished"
			DownloadURL string `json:"download_url"`
		}
		jerr := json.NewDecoder(respP.Body).Decode(&progRes)
		if jerr != nil {
			slog.Warn("Failed to decode progress response", "err", jerr)
		}
		cerr := respP.Body.Close()
		if cerr != nil {
			slog.Warn("Failed to close progress response body", "err", cerr)
		}

		if progRes.Success == 1 || progRes.Text == "Finished" {
			if progRes.DownloadURL == "" {
				return nil, errors.New("loader.do finished but url is empty")
			}
			return &models.VideoResult{
				Title:       initRes.Title,
				DownloadURL: progRes.DownloadURL,
				NeedsMuxing: false,
				Extension:   "mp4",
			}, nil
		}
	}

	return nil, errors.New("loader.do timeout")
}
