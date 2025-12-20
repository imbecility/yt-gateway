package providers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"regexp"
)

// GetVideoTitle tries to get the exact title of the video: fast oEmbed first, then partial HTML parsing.
func GetVideoTitle(client HTTPClient, videoID string) (string, error) {
	title, err := fetchOembedTitle(client, videoID)
	if err == nil && title != "" {
		return title, nil
	}
	slog.Debug("oEmbed title failed, falling back to scraping", "err", err)
	return fetchScrapedTitle(client, videoID)
}

// fetchOembedTitle requests official JSON for iframe-embed video
func fetchOembedTitle(client HTTPClient, videoID string) (string, error) {
	oembedURL := fmt.Sprintf("https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v=%s&format=json", videoID)

	req, _ := http.NewRequest("GET", oembedURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		bcerr := Body.Close()
		if bcerr != nil {
			slog.Warn("failed to close response body", "err", bcerr)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var data struct {
		Title string `json:"title"`
	}
	if jderr := json.NewDecoder(resp.Body).Decode(&data); jderr != nil {
		return "", jderr
	}
	return data.Title, nil
}

// fetchScrapedTitle downloads the first 704KB of the page and looks for the <title>
func fetchScrapedTitle(client HTTPClient, videoID string) (string, error) {
	u := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	req, _ := http.NewRequest("GET", u, nil)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		bcerr := Body.Close()
		if bcerr != nil {
			slog.Warn("failed to close response body", "err", bcerr)
		}
	}(resp.Body)

	scanner := bufio.NewScanner(resp.Body)

	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	re := regexp.MustCompile(`<title>(.*?)(?: - YouTube)?</title>`)

	bytesRead := 0
	const maxBytes = 1024 * 1024

	for scanner.Scan() {
		line := scanner.Text()
		bytesRead += len(line)

		matches := re.FindStringSubmatch(line)
		if len(matches) >= 2 {
			cleanTitle := html.UnescapeString(matches[1])
			return cleanTitle, nil
		}

		if bytesRead > maxBytes {
			break
		}
	}

	return "", fmt.Errorf("title not found in first %d bytes", maxBytes)
}
