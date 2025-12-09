package compressor

import (
	"bytes"
	"context"
	"os"
	"os/exec"

	"github.com/Data-Corruption/stdx/xlog"
)

type HWAccelType int

const (
	HWAccelNone HWAccelType = iota
	HWAccelNvidia
	HWAccelAMDVAAPI
)

type Compressor struct {
	activeHWAccel     HWAccelType
	ffmpegHWAccelArgs []string // usage: "ffmpeg", "-i", inputFile, <ffmpegHWAccelArgs...>, "outputFile"
}

func New(ctx context.Context) *Compressor {
	c := &Compressor{}
	c.activeHWAccel = detectHWAccel()
	xlog.Infof(ctx, "hardware accel detected: %s", c.activeHWAccel.String())

	args := []string{}

	switch c.activeHWAccel {
	case HWAccelNvidia:
		args = append(args,
			"-hwaccel", "cuda",
			"-c:v", "hevc_nvenc",
			"-preset", "p5",
			"-cq", "28",
			"-vf", "scale=-1:720",
		)
	case HWAccelAMDVAAPI:
		args = append(args,
			"-hwaccel", "vaapi",
			"-hwaccel_device", "/dev/dri/renderD128",
			"-vf", "format=nv12,hwupload,scale_vaapi=-1:720",
			"-c:v", "hevc_vaapi",
			"-b:v", "2M",
		)
	default:
		args = append(args,
			"-c:v", "libx264",
			"-preset", "slow",
			"-crf", "28",
			"-vf", "scale=-1:720",
		)
	}

	args = append(args,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "160k",
		"-movflags", "+faststart",
	)

	c.ffmpegHWAccelArgs = args

	return c
}

func (h HWAccelType) String() string {
	switch h {
	case HWAccelNvidia:
		return "nvidia"
	case HWAccelAMDVAAPI:
		return "amd_vaapi"
	case HWAccelNone:
		fallthrough
	default:
		return "none"
	}
}

func detectHWAccel() HWAccelType {
	// Prefer Nvidia if both somehow appear usable
	if hasNvidia() && ffmpegHasEncoder("nvenc") {
		return HWAccelNvidia
	}

	if hasAMDVAAPI() && ffmpegHasEncoder("vaapi") {
		return HWAccelAMDVAAPI
	}

	return HWAccelNone
}

func hasNvidia() bool {
	// Cheap probe: does nvidia-smi work?
	cmd := exec.Command("nvidia-smi", "-L")
	if err := cmd.Run(); err == nil {
		return true
	}

	// Fallback: check for /dev/nvidia0
	if _, err := os.Stat("/dev/nvidia0"); err == nil {
		return true
	}

	return false
}

func hasAMDVAAPI() bool {
	// Basic check: VAAPI render node present
	if _, err := os.Stat("/dev/dri/renderD128"); err != nil {
		return false
	}
	// Optional: check vainfo exists and runs
	cmd := exec.Command("vainfo")
	if err := cmd.Run(); err != nil {
		// Not fatal; VAAPI might still work, but be conservative:
		return false
	}
	return true
}

func ffmpegHasEncoder(substr string) bool {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return false
	}

	return bytes.Contains(out.Bytes(), []byte(substr))
}
