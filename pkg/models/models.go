package models

type VideoResult struct {
	Title       string
	DownloadURL string
	AudioURL    string
	NeedsMuxing bool
	Extension   string
	VideoID     string
}

type APIResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Title   string `json:"title,omitempty"`
	VideoID string `json:"video_id,omitempty"`
	// DirectURL - direct link to the source (if no download was required)
	DirectURL string `json:"direct_url,omitempty"`
	// StreamURL - link to internal API to download a local file (if muxing)
	StreamURL string `json:"stream_url,omitempty"`
	// LocalPath - absolute path (for local integrations)
	LocalPath string `json:"local_path,omitempty"`
}
