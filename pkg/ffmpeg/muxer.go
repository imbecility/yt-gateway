package ffmpeg

import (
	"fmt"
	"os/exec"
)

type Muxer struct {
	BinaryPath string
}

func (m *Muxer) Mux(videoPath, audioPath, outPath string) error {
	cmd := exec.Command(
		m.BinaryPath,
		"-hide_banner",
		"-i", videoPath,
		"-i", audioPath,
		"-c", "copy",
		"-map", "0:v:0",
		"-map", "1:a:0",
		"-sn", "-dn",
		"-map_metadata", "-1",
		"-map_chapters", "-1",
		"-movflags", "faststart",
		"-y",
		outPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %s, output: %s", err, string(output))
	}
	return nil
}
