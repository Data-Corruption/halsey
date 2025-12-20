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

	// Output args for attempt 1 (paired with ffmpegInputArgs).
	ffmpegOutputArgs []string

	// Output args for attempt 2 (paired with ffmpegInputArgsSafe).
	ffmpegOutputArgsSafe []string

	amdRenderNode string
}

func New(ctx context.Context) *Compressor {
	c := &Compressor{}

	c.activeHWAccel, c.amdRenderNode = detectHWAccel()
	xlog.Infof(ctx, "hardware accel detected: %s", c.activeHWAccel.String())

	var inArgs, inSafeArgs []string
	var outArgs, outSafeArgs []string

	// NVENC / SW filters:
	const sharedVF = "scale=-2:min(1080\\,ih)"

	// VAAPI filters:
	// - hw: frames are already VAAPI hwframes -> do NOT hwupload
	// - sw: frames are system-memory -> upload then scale on VAAPI
	const vaapiVF_hw = "scale_vaapi=w=-2:h='min(1080\\,ih)':format=nv12"
	const vaapiVF_sw = "format=nv12,hwupload,scale_vaapi=w=-2:h='min(1080\\,ih)':format=nv12"

	switch c.activeHWAccel {
	case HWAccelNvidia:
		// Keep output args identical between attempts so retry truly only changes decode path.
		// (Yes, this may trigger hwdownload for scaling, but it's robust.)
		inArgs = append(inArgs, "-hwaccel", "cuda")
		outArgs = append(outArgs,
			"-vf", sharedVF,
			"-c:v", "av1_nvenc",
			"-preset", "p5",
			"-tune", "hq",
			"-rc", "vbr",
			"-b:v", "0",
			"-cq:v", "50",
			// optional consistency with SVT path:
			"-g", "240",
			// NVENC can handle format internally; keep yuv420p for broad compatibility in WebM.
			"-pix_fmt", "yuv420p",
		)
		outSafeArgs = append(outSafeArgs, outArgs...)

	case HWAccelAMDVAAPI:
		if c.amdRenderNode == "" {
			c.amdRenderNode = "/dev/dri/renderD128"
		}

		// Keep VAAPI device selection in BOTH modes.
		inSafeArgs = append(inSafeArgs, "-vaapi_device", c.amdRenderNode)

		// Attempt 1 requests VAAPI hw decode and ensures VAAPI hwframes are produced.
		inArgs = append(inArgs,
			"-vaapi_device", c.amdRenderNode,
			"-hwaccel", "vaapi",
			"-hwaccel_output_format", "vaapi",
		)

		// Attempt 1 output: input frames are VAAPI hwframes already.
		outArgs = append(outArgs,
			"-vf", vaapiVF_hw,
			"-c:v", "av1_vaapi",
			"-q:v", "160",
			"-g", "240",
			// DO NOT force -pix_fmt yuv420p here; keep frames on the VAAPI path (nv12).
		)

		// Attempt 2 output: input frames are system-memory.
		outSafeArgs = append(outSafeArgs,
			"-vf", vaapiVF_sw,
			"-c:v", "av1_vaapi",
			"-q:v", "160",
			"-g", "240",
		)

	default:
		// Pure software path; same args for both attempts.
		outArgs = append(outArgs,
			"-vf", sharedVF,
			"-c:v", "libsvtav1",
			"-preset", "3",
			"-crf", "48",
			"-g", "240",
			"-pix_fmt", "yuv420p",
		)
		outSafeArgs = append(outSafeArgs, outArgs...)
	}

	// Audio + container options: append to both output arg sets.
	commonTail := []string{
		"-c:a", "libopus",
		"-b:a", "128k",
		"-ac", "2",
		"-map", "0:v:0?",
		"-map", "0:a?",
		"-sn",
		"-f", "webm",
	}

	outArgs = append(outArgs, commonTail...)
	outSafeArgs = append(outSafeArgs, commonTail...)

	c.ffmpegInputArgs = inArgs
	c.ffmpegInputArgsSafe = inSafeArgs
	c.ffmpegOutputArgs = outArgs
	c.ffmpegOutputArgsSafe = outSafeArgs
	return c
}

func (c *Compressor) GetHWAccel() HWAccelType {
	return c.activeHWAccel
}

// Video compresses a video file. Output is AV1 video with Opus audio in WebM.
func (c *Compressor) Video(ctx context.Context, inputFile, outputFile string, timeout time.Duration) error {
	run := func(dCtx context.Context, inputArgs, outputArgs []string) (string, error) {
		args := []string{
			"-hide_banner",
			"-nostdin",
			"-nostats",
			"-loglevel", "warning",
			"-y",
		}

		// Input options must come before -i.
		args = append(args, inputArgs...)
		args = append(args, "-i", inputFile)

		// Output options after input.
		args = append(args, outputArgs...)
		args = append(args, outputFile)

		cmd := exec.CommandContext(dCtx, "ffmpeg", args...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		xlog.Debugf(ctx, "Running ffmpeg command: ffmpeg %v", args)
		err := cmd.Run()
		return out.String(), err
	}

	// Attempt 1: try with HW-decode requests (if any)
	dCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := run(dCtx, c.ffmpegInputArgs, c.ffmpegOutputArgs)
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

	out2, err2 := run(dCtx2, c.ffmpegInputArgsSafe, c.ffmpegOutputArgsSafe)
	if err2 == nil {
		xlog.Infof(ctx, "ffmpeg succeeded after retry without hw-decode")
		return nil
	}

	xlog.Errorf(ctx, "ffmpeg error (safe retry): %v, output: %s", err2, out2)
	return classifyError(dCtx2, err2, out2)
}

// Image compresses an image file to AVIF format. Output path should have .avif extension.
func (c *Compressor) Image(ctx context.Context, inputFile, outputFile string, timeout time.Duration) error {
	run := func(dCtx context.Context) (string, error) {
		args := []string{
			"-hide_banner",
			"-nostdin",
			"-nostats",
			"-loglevel", "warning",
			"-y",
			"-i", inputFile,
			"-c:v", "libaom-av1",
			"-crf", "30",
			"-b:v", "0",
			outputFile,
		}

		cmd := exec.CommandContext(dCtx, "ffmpeg", args...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		xlog.Debugf(ctx, "Running ffmpeg command: ffmpeg %v", args)
		err := cmd.Run()
		return out.String(), err
	}

	dCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, err := run(dCtx)
	if err == nil {
		return nil
	}
	xlog.Errorf(ctx, "ffmpeg image error: %v, output: %s", err, out)

	return classifyError(dCtx, err, out)
}

// classifyError inspects ffmpeg output and context to determine the cause of failure.
// This is a shared helper used by both Video() and Image() compression methods.
func classifyError(dCtx context.Context, err error, output string) *CompressError {
	// Check for timeout first.
	if errors.Is(dCtx.Err(), context.DeadlineExceeded) {
		return &CompressError{Cause: CauseTimeout, Err: err, Output: output}
	}

	outLower := strings.ToLower(output)

	// IO-related errors first (these aren't decode/encode problems).
	ioIndicators := []string{
		"no such file",
		"does not exist",
		"could not open",
		"permission denied",
	}
	for _, indicator := range ioIndicators {
		if strings.Contains(outLower, indicator) {
			return &CompressError{Cause: CauseUnknown, Err: err, Output: output}
		}
	}

	// Decode-related errors.
	decodeIndicators := []string{
		"invalid data found",
		"could not find codec",
		"decoder",
		"demuxer",
		"error while decoding",
		"moov atom not found",
		"corrupt",
	}
	for _, indicator := range decodeIndicators {
		if strings.Contains(outLower, indicator) {
			return &CompressError{Cause: CauseDecode, Err: err, Output: output}
		}
	}

	// Encode-related errors.
	encodeIndicators := []string{
		"encoder",
		"nvenc",
		"vaapi",
		"amf",
		"libaom",
		"av1",
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
	if hasNvidia() && ffmpegHasEncoder("av1_nvenc") {
		return HWAccelNvidia, ""
	}

	if ffmpegHasEncoder("av1_vaapi") {
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
	if len(matches) > 0 {
		return matches[0], true
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

// Probe VAAPI by encoding 1 frame of black video with av1_vaapi.
func ffmpegVAAPIProbe(renderNode string) error {
	args := []string{
		"-hide_banner", "-nostdin",
		"-vaapi_device", renderNode,
		"-f", "lavfi",
		"-i", "color=c=black:s=128x128:r=1",
		"-frames:v", "1",
		"-vf", "format=nv12,hwupload",
		"-c:v", "av1_vaapi",
		"-q:v", "25",
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
