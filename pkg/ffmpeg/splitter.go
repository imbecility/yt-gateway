package ffmpeg

import (
	"bytes"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Split checks the file size. If it is larger than maxSize, the file is cut into pieces.
// It uses a recursive approach: if a part is too large, it is divided into halves.
func (m *Muxer) Split(inputPath string, maxSize int64) ([]string, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := info.Size()

	if fileSize <= maxSize {
		return []string{inputPath}, nil
	}

	slog.Info("File exceeds size limit, splitting...",
		"file_size_mb", fileSize/1024/1024,
		"limit_mb", maxSize/1024/1024)

	durationSec, err := m.getDuration(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get duration: %w", err)
	}

	bytesPerSec := float64(fileSize) / durationSec
	chunkDuration := (float64(maxSize) * 0.9) / bytesPerSec

	if chunkDuration < 10.0 {
		chunkDuration = 10.0
	}

	ext := filepath.Ext(inputPath)
	baseName := strings.TrimSuffix(inputPath, ext)

	tempDir := filepath.Join(filepath.Dir(inputPath), "split_temp")
	_ = os.MkdirAll(tempDir, 0755)
	defer func(path string) {
		rmerr := os.RemoveAll(path)
		if rmerr != nil {
			slog.Error("failed to remove temp dir", "path", path, "err", rmerr)
		}
	}(tempDir)

	var tempFiles []string

	var processSegment func(start, dur float64) error
	processSegment = func(start, dur float64) error {
		tmpName := filepath.Join(tempDir, fmt.Sprintf("chunk_%.2f_%.2f%s", start, dur, ext))

		err := m.cutSegment(inputPath, tmpName, start, dur)
		if err != nil {
			return err
		}

		stat, err := os.Stat(tmpName)
		if err != nil {
			return err
		}

		if stat.Size() > maxSize && dur > 1.0 {
			slog.Warn("Chunk too big, resplitting...", "size_mb", stat.Size()/1024/1024, "dur", dur)
			_ = os.Remove(tmpName)

			half := dur / 2
			if err := processSegment(start, half); err != nil {
				return err
			}

			if start+half < durationSec {
				remDur := half
				if start+half+remDur > durationSec {
					remDur = durationSec - (start + half)
				}
				if remDur > 0 {
					if err := processSegment(start+half, remDur); err != nil {
						return err
					}
				}
			}
			return nil
		}

		tempFiles = append(tempFiles, tmpName)
		return nil
	}

	totalPartsNaive := int(math.Ceil(durationSec / chunkDuration))

	for i := 0; i < totalPartsNaive; i++ {
		start := float64(i) * chunkDuration
		dur := chunkDuration

		if start+dur > durationSec {
			dur = durationSec - start
		}

		if dur <= 0.1 {
			break
		}

		if err := processSegment(start, dur); err != nil {
			return nil, err
		}
	}

	var finalPaths []string

	for i, tmpFile := range tempFiles {
		finalName := fmt.Sprintf("%s_part%03d%s", baseName, i+1, ext)

		err := os.Rename(tmpFile, finalName)
		if err != nil {
			return nil, fmt.Errorf("failed to rename final part: %w", err)
		}

		finalPaths = append(finalPaths, finalName)
	}

	_ = os.Remove(inputPath)

	return finalPaths, nil
}

func (m *Muxer) getDuration(path string) (float64, error) {
	cmd := exec.Command(m.BinaryPath, "-hide_banner", "-i", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	_ = cmd.Run()

	output := stderr.String()

	re := regexp.MustCompile(`Duration: (\d{2}):(\d{2}):(\d{2}\.\d{2})`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 4 {
		return 0, fmt.Errorf("duration not found in ffmpeg output")
	}

	hours, _ := strconv.ParseFloat(matches[1], 64)
	mins, _ := strconv.ParseFloat(matches[2], 64)
	secs, _ := strconv.ParseFloat(matches[3], 64)

	totalSec := hours*3600 + mins*60 + secs
	return totalSec, nil
}

func (m *Muxer) cutSegment(input, output string, start, duration float64) error {
	args := []string{
		"-hide_banner",
		"-ss", fmt.Sprintf("%.2f", start),
		"-i", input,
		"-t", fmt.Sprintf("%.2f", duration),
		"-c", "copy",
		"-map", "0",
		"-avoid_negative_ts", "1",
		"-sn", "-dn",
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-movflags", "faststart",
		"-y",
		output,
	}

	cmd := exec.Command(m.BinaryPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %s, ffmpeg output: %s", err, string(out))
	}
	st, sterr := os.Stat(output)
	if sterr != nil || st.Size() == 0 {
		return fmt.Errorf("ffmpeg produced empty file or no file")
	}
	return nil
}
