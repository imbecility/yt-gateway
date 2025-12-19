package ffmpeg

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/imbecility/yt-gateway/pkg/providers"
)

const (
	UrlNanoLinux   = "https://github.com/imbecility/yt-gateway/releases/download/ffmpeg_git-2025-12-18-78c75d5/ffmpeg_nano"
	UrlNanoWindows = "https://github.com/imbecility/yt-gateway/releases/download/ffmpeg_git-2025-12-18-78c75d5/ffmpeg_nano.exe"
)

func EnsureBinary(client providers.HTTPClient, requestedPath string) (string, error) {
	if isWorking(requestedPath) {
		slog.Debug("FFmpeg found and working", "path", requestedPath)
		return requestedPath, nil
	}

	slog.Warn("FFmpeg not found or invalid. Attempting to download embedded version...", "path", requestedPath)

	var downloadUrl string
	var fileName string

	switch runtime.GOOS {
	case "windows":
		downloadUrl = UrlNanoWindows
		fileName = "ffmpeg_nano.exe"
	case "linux", "darwin":
		downloadUrl = UrlNanoLinux
		fileName = "ffmpeg_nano"
	default:
		return "", fmt.Errorf("auto-download not supported for OS: %s", runtime.GOOS)
	}

	cwd, _ := os.Getwd()
	localPath := filepath.Join(cwd, fileName)

	if _, err := os.Stat(localPath); err == nil {
		if isWorking(localPath) {
			slog.Info("Found local nano ffmpeg", "path", localPath)
			return localPath, nil
		}
		remferr := os.Remove(localPath)
		if remferr != nil {
			slog.Warn("Failed to delete a broken executable file of ffmpeg.", "path", localPath, "err", remferr)
		}
	}

	slog.Info("Downloading ffmpeg nano...", "url", downloadUrl)
	if err := downloadFile(client, downloadUrl, localPath); err != nil {
		return "", fmt.Errorf("failed to download ffmpeg: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(localPath, 0755); err != nil {
			return "", fmt.Errorf("failed to chmod ffmpeg: %w", err)
		}
	}

	if isWorking(localPath) {
		slog.Info("FFmpeg installed successfully", "path", localPath)
		return localPath, nil
	}

	return "", fmt.Errorf("downloaded ffmpeg is not working")
}

func isWorking(path string) bool {
	cmd := exec.Command(path, "-version")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func downloadFile(client providers.HTTPClient, url string, dest string) error {
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		cerr := Body.Close()
		if cerr != nil {
			slog.Warn("Failed to close response body", "error", cerr)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("http status: %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		cerr := out.Close()
		if cerr != nil {
			slog.Warn("Failed to close file", "error", cerr)
		}
	}(out)

	_, err = io.Copy(out, resp.Body)
	return err
}
