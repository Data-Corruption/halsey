package compress

import (
	"bytes"
	"context"
	"os/exec"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
)

func Video(ctx context.Context, inputFile, outputFile string, timeout time.Duration) error {
	dCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"-y", // overwrite output
		"-i", inputFile,
	}
	args = append(args, ffmpegHWAccelArgs...)
	args = append(args, outputFile)

	cmd := exec.CommandContext(dCtx, "ffmpeg", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	xlog.Debugf(ctx, "Running ffmpeg command: ffmpeg %v", args)
	if err := cmd.Run(); err != nil {
		xlog.Errorf(ctx, "ffmpeg error: %v, output: %s", err, out.String())
		return err
	}

	return nil
}
