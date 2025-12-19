package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/imbecility/yt-gateway/pkg/downloader"
	"github.com/imbecility/yt-gateway/pkg/gateway"
	"github.com/imbecility/yt-gateway/pkg/models"
	"github.com/imbecility/yt-gateway/pkg/utils"
)

type Server struct {
	Port            int
	Gateway         *gateway.Service
	Downloader      *downloader.Downloader
	Host            string
	mu              sync.Mutex
	activeDownloads map[string]int
}

func (s *Server) Start(enableWeb bool) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/download", s.handleAPIDownload)
	mux.HandleFunc("/files/", s.handleFileDownload)

	if enableWeb {
		mux.HandleFunc("/", s.handleWebIndex)
	}

	addr := fmt.Sprintf(":%d", s.Port)
	fullAddr := fmt.Sprintf("http://localhost:%d", s.Port)
	slog.Info("Starting API server", "addr", fullAddr, "web_ui", enableWeb)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleAPIDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	vidID := utils.ExtractVideoID(req.URL)
	if vidID == "" {
		s.respondJSON(w, models.APIResponse{Success: false, Error: "Invalid URL"})
		return
	}
	fullURL := "https://www.youtube.com/watch?v=" + vidID

	slog.Info("API request received", "vid", vidID, "remote", r.RemoteAddr)

	res, provName, err := s.Gateway.GetLinkWithRetries(fullURL)
	if err != nil {
		slog.Error("Processing failed", "vid", vidID, "err", err)
		s.respondJSON(w, models.APIResponse{Success: false, Error: "All providers failed"})
		return
	}
	res.VideoID = vidID

	response := models.APIResponse{
		Success: true,
		Title:   res.Title,
		VideoID: vidID,
	}

	if res.NeedsMuxing {
		slog.Info("Muxing required for API", "provider", provName)
		localPath, err := s.Downloader.DownloadAndMux(res)
		if err != nil {
			slog.Error("Download/Mux failed", "err", err)
			s.respondJSON(w, models.APIResponse{Success: false, Error: err.Error()})
			return
		}

		filename := filepath.Base(localPath)
		response.LocalPath, _ = filepath.Abs(localPath)
		response.StreamURL = fmt.Sprintf("%s/files/%s", s.Host, filename)
	} else {
		response.DirectURL = res.DownloadURL
	}

	s.respondJSON(w, response)
}

func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/files/")
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	path := filepath.Join(s.Downloader.OutputDir, filename)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "File not found or expired", http.StatusNotFound)
		return
	}

	s.trackFileStart(filename)

	defer s.trackFileEnd(filename)

	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "File access error", http.StatusInternalServerError)
		return
	}
	defer func(file *os.File) {
		cerr := file.Close()
		if cerr != nil {
			slog.Error("Error closing file", "err", cerr)
		}
	}(file)

	slog.Info("Serving file via API", "file", filename, "remote", r.RemoteAddr)

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	http.ServeContent(w, r, filename, time.Now(), file)
}

func (s *Server) BackgroundCleaner(ttl time.Duration) {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		files, err := os.ReadDir(s.Downloader.OutputDir)
		if err != nil {
			slog.Error("Cleaner cant read dir", "err", err)
			continue
		}

		for _, f := range files {
			name := f.Name()

			if strings.Contains(name, "_tmp") || strings.HasSuffix(name, ".part") {
				continue
			}

			if s.isFileBusy(name) {
				slog.Debug("Skipping busy file", "file", name)
				continue
			}

			info, _ := f.Info()
			if time.Since(info.ModTime()) > ttl {
				fullPath := filepath.Join(s.Downloader.OutputDir, name)
				err := os.Remove(fullPath)
				if err != nil {
					slog.Error("Failed to remove file", "err", err)
				} else {
					slog.Debug("Cleaned up old file", "file", name)
				}
			}
		}
	}
}

func (s *Server) trackFileStart(filename string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeDownloads[filename]++
}

func (s *Server) trackFileEnd(filename string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeDownloads[filename]--
	if s.activeDownloads[filename] <= 0 {
		delete(s.activeDownloads, filename)
	}
}

func (s *Server) isFileBusy(filename string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeDownloads[filename] > 0
}

func (s *Server) handleWebIndex(w http.ResponseWriter, r *http.Request) {

	t, _ := template.New("index").Parse(tmpl)
	err := t.Execute(w, nil)
	if err != nil {
		slog.Error("Template execution failed", "error", err, "remote", r.RemoteAddr)
	}
}

func (s *Server) respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	jerr := json.NewEncoder(w).Encode(data)
	if jerr != nil {
		slog.Error("JSON encoding failed", "error", jerr)
	}
}
