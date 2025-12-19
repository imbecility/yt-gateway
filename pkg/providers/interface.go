package providers

import (
	"net/http"

	"github.com/imbecility/yt-gateway/pkg/models"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Provider interface {
	Name() string
	GetLink(youtubeURL string) (*models.VideoResult, error)
}
