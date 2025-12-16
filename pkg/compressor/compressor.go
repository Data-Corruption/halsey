// Package compressor provides a simple interface for compressing video files.
//
// Example Usage:
//
// err := compressor.Video(ctx, input, output, 5*time.Minute)
//
//	if err != nil {
//	    switch {
//	    case compressor.IsTimeout(err):
//	        // Video too long/complex - link source instead
//	        return linkSourceURL(sourceURL)
//	    case compressor.IsDecode(err):
//	        // Bad input file - link source, maybe warn user
//	        return linkSourceURL(sourceURL)
//	    case compressor.IsEncode(err):
//	        // HW encoder issue - could retry with different settings
//	        // or just link source
//	        return linkSourceURL(sourceURL)
//	    default:
//	        // Unknown - log and link source
//	        log.Error(err)
//	        return linkSourceURL(sourceURL)
//	    }
//	}
package compressor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
)

// ErrorCause describes why compression failed.
type ErrorCause string

const (
	// CauseTimeout indicates the operation exceeded the timeout.
	CauseTimeout ErrorCause = "timeout"
	// CauseDecode indicates the input file couldn't be decoded.
	CauseDecode ErrorCause = "decode"
	// CauseEncode indicates encoding failed (e.g., HW encoder error).
	CauseEncode ErrorCause = "encode"
	// CauseUnknown indicates an unclassified failure.
	CauseUnknown ErrorCause = "unknown"
)

// CompressError wraps compression failures with context about the cause.
type CompressError struct {
	Cause  ErrorCause
	Err    error
	Output string // ffmpeg stderr/stdout for debugging
}

func (e *CompressError) Error() string {
	return fmt.Sprintf("compression failed (%s): %v", e.Cause, e.Err)
}

func (e *CompressError) Unwrap() error {
	return e.Err
}

// IsTimeout returns true if the error was caused by a timeout.
func IsTimeout(err error) bool {
	var ce *CompressError
	if errors.As(err, &ce) {
		return ce.Cause == CauseTimeout
	}
	return false
}

// IsDecode returns true if the error was caused by decode failure.
func IsDecode(err error) bool {
	var ce *CompressError
	if errors.As(err, &ce) {
		return ce.Cause == CauseDecode
	}
	return false
}

// IsEncode returns true if the error was caused by encode failure.
func IsEncode(err error) bool {
	var ce *CompressError
	if errors.As(err, &ce) {
		return ce.Cause == CauseEncode
	}
	return false
}

type HWAccelType int

const (
	HWAccelNone HWAccelType = iota
	HWAccelNvidia
	HWAccelAMDVAAPI
)

type Compressor struct {
	activeHWAccel HWAccelType

	// Input args that request HW decode (may fail on weird inputs).
	ffmpegInputArgs []string

	// Safe input args (no HW decode request) used for retry.
	ffmpegInputArgsSafe []string

	// Output options
	ffmpegOutputArgs []string

	amdRenderNode string
}

func New(ctx context.Context) *Compressor {
	c := &Compressor{}

	c.activeHWAccel, c.amdRenderNode = detectHWAccel()
	xlog.Infof(ctx, "hardware accel detected: %s", c.activeHWAccel.String())

	var inArgs, inSafeArgs, outArgs []string

	// Avoid accidental upscaling:
	// scale=-2:min(1080\,ih) => preserve aspect, even width, cap height at 1080.
	// Note: the comma inside min() must be escaped for ffmpeg's filter parser.
	const scale1080NoUpscale = "scale=-2:min(1080\\,ih)"
	const vaapiScale1080NoUpscale = "format=nv12,hwupload,scale_vaapi=-2:min(1080\\,ih)"

	switch c.activeHWAccel {
	case HWAccelNvidia:
		// Try HW decode first; retry will drop this.
		inArgs = append(inArgs, "-hwaccel", "cuda")

		outArgs = append(outArgs,
			"-c:v", "h264_nvenc",
			"-preset", "p5",
			"-cq", "23",
			"-vf", scale1080NoUpscale,
		)

	case HWAccelAMDVAAPI:
		if c.amdRenderNode == "" {
			c.amdRenderNode = "/dev/dri/renderD128"
		}

		// Keep VAAPI device selection in BOTH modes.
		// "Try" mode requests VAAPI decode; safe mode does not.
		inSafeArgs = append(inSafeArgs, "-vaapi_device", c.amdRenderNode)
		inArgs = append(inArgs,
			"-vaapi_device", c.amdRenderNode,
			"-hwaccel", "vaapi",
		)

		outArgs = append(outArgs,
			"-vf", vaapiScale1080NoUpscale,
			"-c:v", "h264_vaapi",
			"-b:v", "6M",
			"-maxrate", "8M",
			"-bufsize", "16M",
		)

	default:
		outArgs = append(outArgs,
			"-c:v", "libx264",
			"-preset", "slow",
			"-crf", "28",
			"-vf", scale1080NoUpscale,
		)
	}

	outArgs = append(outArgs,
		"-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "160k",
		"-movflags", "+faststart",
		"-map", "0:v:0",
		"-map", "0:a?",
		"-sn",
	)

	c.ffmpegInputArgs = inArgs
	c.ffmpegInputArgsSafe = inSafeArgs
	c.ffmpegOutputArgs = outArgs
	return c
}

func (c *Compressor) GetHWAccel() HWAccelType {
	return c.activeHWAccel
}

func (c *Compressor) Video(ctx context.Context, inputFile, outputFile string, timeout time.Duration) error {
	run := func(dCtx context.Context, inputArgs []string) (string, error) {
		args := []string{
			"-hide_banner",
			"-nostdin",
			"-nostats", // modern flag, if issue just remove
			"-loglevel", "warning",
			"-y",
		}

		// Input options must come before -i.
		args = append(args, inputArgs...)
		args = append(args, "-i", inputFile)

		// Output options after input.
		args = append(args, c.ffmpegOutputArgs...)
		args = append(args, outputFile)

		cmd := exec.CommandContext(dCtx, "ffmpeg", args...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		xlog.Debugf(ctx, "Running ffmpeg command: ffmpeg %v", args)
		err := cmd.Run()
		return out.String(), err
	}

	// classifyError inspects ffmpeg output and context to determine the cause.
	classifyError := func(dCtx context.Context, err error, output string) *CompressError {
		// Check for timeout first.
		if errors.Is(dCtx.Err(), context.DeadlineExceeded) {
			return &CompressError{Cause: CauseTimeout, Err: err, Output: output}
		}

		// Check ffmpeg output for decode-related errors.
		outLower := strings.ToLower(output)
		decodeIndicators := []string{
			"invalid data found",
			"could not find codec",
			"decoder",
			"demuxer",
			"no such file",
			"does not exist",
			"error while decoding",
			"corrupt",
			"invalid",
			"moov atom not found",
			"could not open",
		}
		for _, indicator := range decodeIndicators {
			if strings.Contains(outLower, indicator) {
				return &CompressError{Cause: CauseDecode, Err: err, Output: output}
			}
		}

		// Check for encode-related errors.
		encodeIndicators := []string{
			"encoder",
			"nvenc",
			"vaapi",
			"h264_",
			"encoding",
			"hwupload",
			"filter",
			"scale",
		}
		for _, indicator := range encodeIndicators {
			if strings.Contains(outLower, indicator) {
				return &CompressError{Cause: CauseEncode, Err: err, Output: output}
			}
		}

		return &CompressError{Cause: CauseUnknown, Err: err, Output: output}
	}

	// Attempt 1: try with HW-decode requests (if any)
	dCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := run(dCtx, c.ffmpegInputArgs)
	if err == nil {
		return nil
	}
	xlog.Errorf(ctx, "ffmpeg error (hw-decode attempt): %v, output: %s", err, out)

	// Retry only if we actually requested HW decode (otherwise pointless).
	if len(c.ffmpegInputArgs) == 0 {
		return classifyError(dCtx, err, out)
	}
	if c.activeHWAccel != HWAccelAMDVAAPI && c.activeHWAccel != HWAccelNvidia {
		return classifyError(dCtx, err, out)
	}

	// Attempt 2: safe input args (no hw decode request)
	dCtx2, cancel2 := context.WithTimeout(ctx, timeout)
	defer cancel2()

	out2, err2 := run(dCtx2, c.ffmpegInputArgsSafe)
	if err2 == nil {
		xlog.Infof(ctx, "ffmpeg succeeded after retry without hw-decode")
		return nil
	}

	xlog.Errorf(ctx, "ffmpeg error (safe retry): %v, output: %s", err2, out2)
	return classifyError(dCtx2, err2, out2)
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

func detectHWAccel() (HWAccelType, string) {
	// Check for the exact encoders we use.
	if hasNvidia() && ffmpegHasEncoder("h264_nvenc") {
		return HWAccelNvidia, ""
	}

	if ffmpegHasEncoder("h264_vaapi") {
		if node, ok := pickVAAPIRenderNode(); ok {
			// Probe by default; disable with VIDEO_VAAPI_PROBE=0
			if strings.TrimSpace(os.Getenv("VIDEO_VAAPI_PROBE")) != "0" {
				if err := ffmpegVAAPIProbe(node); err != nil {
					return HWAccelNone, ""
				}
			}
			return HWAccelAMDVAAPI, node
		}
	}

	return HWAccelNone, ""
}

func hasNvidia() bool {
	// Cheap probe: does nvidia-smi work?
	if err := exec.Command("nvidia-smi", "-L").Run(); err == nil {
		return true
	}
	// Fallback: check for /dev/nvidia0
	if _, err := os.Stat("/dev/nvidia0"); err == nil {
		return true
	}
	return false
}

func pickVAAPIRenderNode() (string, bool) {
	// Allow explicit override (useful when multiple GPUs exist)
	if v := strings.TrimSpace(os.Getenv("VIDEO_RENDER_NODE")); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v, true
		}
		return "", false
	}

	// Most common: renderD128
	const defaultNode = "/dev/dri/renderD128"
	if _, err := os.Stat(defaultNode); err == nil {
		return defaultNode, true
	}

	// Fallback: scan all /dev/dri/renderD* and pick first existing.
	matches, _ := filepath.Glob("/dev/dri/renderD*")
	sort.Strings(matches)
	for _, m := range matches {
		if _, err := os.Stat(m); err == nil {
			return m, true
		}
	}

	return "", false
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

// Probe VAAPI by generating 1 frame of black video and trying to encode it with h264_vaapi.
func ffmpegVAAPIProbe(renderNode string) error {
	args := []string{
		"-hide_banner", "-nostdin",
		"-vaapi_device", renderNode,
		"-f", "lavfi",
		"-i", "color=c=black:s=64x64:r=1",
		"-frames:v", "1",
		"-vf", "format=nv12,hwupload",
		"-c:v", "h264_vaapi",
		"-f", "null", "-",
	}
	cmd := exec.Command("ffmpeg", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vaapi probe failed: %w output=%s", err, out.String())
	}
	return nil
}
