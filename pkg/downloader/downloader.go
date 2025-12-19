package downloader

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/imbecility/yt-gateway/pkg/ffmpeg"
	"github.com/imbecility/yt-gateway/pkg/models"
	"github.com/imbecility/yt-gateway/pkg/providers"
)

type Downloader struct {
	Client       providers.HTTPClient
	FFmpegPath   string
	OutputDir    string
	ShowProgress bool
}

type ProgressWriter struct {
	Total      int64
	Downloaded int64
	LastPrint  time.Time
	Logger     *slog.Logger
	Type       string // "Audio", "Video", "File"
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Downloaded += int64(n)

	if time.Since(pw.LastPrint) > 100*time.Millisecond {
		pw.printProgress()
		pw.LastPrint = time.Now()
	}
	return n, nil
}

func (pw *ProgressWriter) printProgress() {
	mb := float64(pw.Downloaded) / 1024 / 1024

	if pw.Total > 0 {
		percent := float64(pw.Downloaded) / float64(pw.Total) * 100
		totalMb := float64(pw.Total) / 1024 / 1024
		fmt.Printf("\r[%s] %.2f%% (%.2f/%.2f MB)   ", pw.Type, percent, mb, totalMb)
	} else {
		fmt.Printf("\r[%s] Downloading... %.2f MB   ", pw.Type, mb)
	}
}

func (d *Downloader) DownloadAndMux(res *models.VideoResult) (string, error) {
	fileName := res.VideoID + "." + res.Extension
	finalPath := filepath.Join(d.OutputDir, fileName)

	if !res.NeedsMuxing {
		slog.Debug("Starting direct download", "url", res.DownloadURL)
		if err := d.downloadFile(res.DownloadURL, finalPath, "File"); err != nil {
			return "", err
		}
		if d.ShowProgress {
			fmt.Println()
		}
		return finalPath, nil
	}

	slog.Debug("Starting muxing download", "id", res.VideoID)
	vidTmp := filepath.Join(d.OutputDir, fmt.Sprintf("%s_vid_tmp.mp4", res.VideoID))
	audTmp := filepath.Join(d.OutputDir, fmt.Sprintf("%s_aud_tmp.m4a", res.VideoID))

	var wg sync.WaitGroup
	var errVideo, errAudio error

	wg.Add(2)
	go func() {
		defer wg.Done()
		errVideo = d.downloadFile(res.DownloadURL, vidTmp, "Video")
	}()

	go func() {
		defer wg.Done()
		errAudio = d.downloadFile(res.AudioURL, audTmp, "Audio")
	}()

	wg.Wait()

	if d.ShowProgress {
		fmt.Println()
	}

	if errVideo != nil || errAudio != nil {
		vfderr := os.Remove(vidTmp)
		if vfderr != nil {
			slog.Error("Error removing video file", "error", vfderr)
		}
		afderr := os.Remove(audTmp)
		if afderr != nil {
			slog.Error("Error removing audio file", "error", afderr)
		}
		return "", fmt.Errorf("download failed: video_err=%v, audio_err=%v", errVideo, errAudio)
	}

	slog.Debug("Streams downloaded, starting ffmpeg muxing")
	muxer := ffmpeg.Muxer{BinaryPath: d.FFmpegPath}
	if err := muxer.Mux(vidTmp, audTmp, finalPath); err != nil {
		return "", fmt.Errorf("muxing error: %w", err)
	}

	vfderr := os.Remove(vidTmp)
	if vfderr != nil {
		slog.Error("Error removing video file", "error", vfderr)
	}
	afderr := os.Remove(audTmp)
	if afderr != nil {
		slog.Error("Error removing audio file", "error", afderr)
	}
	return finalPath, nil
}

func (d *Downloader) downloadFile(url string, fpath string, streamType string) error {
	out, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		ferr := out.Close()
		if ferr != nil {
			slog.Error("Error closing file", "error", ferr)
		}
	}(out)

	req, _ := http.NewRequest("GET", url, nil)

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		cerr := Body.Close()
		if cerr != nil {
			slog.Warn("Error closing response body", "error", cerr)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("http status: %d", resp.StatusCode)
	}

	var source io.Reader = resp.Body

	if d.ShowProgress {
		pw := &ProgressWriter{
			Total:     resp.ContentLength, // can be -1
			Type:      streamType,
			LastPrint: time.Now(),
		}
		source = &progressReaderWrapper{
			Reader: resp.Body,
			Pw:     pw,
		}
	}

	_, err = io.Copy(out, source)
	return err
}

type progressReaderWrapper struct {
	io.Reader
	Pw *ProgressWriter
}

func (p *progressReaderWrapper) Read(b []byte) (int, error) {
	n, err := p.Reader.Read(b)
	if n > 0 {
		_, perr := p.Pw.Write(b[:n])
		if perr != nil {
			return 0, perr
		}
	}
	return n, err
}
