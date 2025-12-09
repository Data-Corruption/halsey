// Package download implements a three-stage media retrieval pipeline:
//
//  1. ParseDomain classifies the incoming link so we know which extractor to use.
//  2. Domain-specific extractors resolve high-level resources (posts, threads, etc.)
//     into direct media URLs; YouTube/Shorts skip this step and go straight to yt-dlp.
//  3. DownloadMedia downloads the media URL to a temp file using the appropriate
//     tool (e.g., ffmpeg, curl) determined by analyzing the URL for a format type.
//
// Doing it like this keeps file-level downloads simple to reason about, yet allows the
// package to accept richer inputs and makes future extractors easy to add.
package download

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultTimeout = 5 * time.Minute

var ErrTooManyRequests = errors.New("too many requests, try again later")

// DownloadMedia parses the URL into a DownloadPlan and executes it.
// Returns the path to the downloaded file or an error.
func DownloadMedia(rawURL, userAgent string, timeout time.Duration) (string, error) {
	plan, err := ParseMediaURL(rawURL)
	if err != nil {
		return "", err
	}
	return DownloadWithPlan(plan, userAgent, timeout)
}

// DownloadWithPlan executes a previously parsed download plan.
// Returns the path to the downloaded file or an error.
func DownloadWithPlan(plan DownloadPlan, userAgent string, timeout time.Duration) (string, error) {
	if err := plan.Validate(); err != nil {
		return "", err
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch plan.Strategy {
	case StrategyFFmpeg:
		return fetchWithTempFile(ctx, plan, "ffmpeg", userAgent, runFFmpeg)
	case StrategyDirect:
		return fetchWithTempFile(ctx, plan, "curl", userAgent, runCurl)
	default:
		return "", fmt.Errorf("unsupported download strategy: %s", plan.Strategy)
	}
}

// YtDLP downloads YouTube media to a temp file, moves it to a safe location, and returns the path.
// The caller is responsible for removing the returned file when done.
func YtDLP(ctx context.Context, rawURL string, timeout time.Duration) (string, error) {
	if err := ensureTool("yt-dlp"); err != nil {
		return "", err
	}

	// create isolated temp dir
	tmpDir, err := os.MkdirTemp("", "yt-dlp-build-")
	if err != nil {
		return "", fmt.Errorf("mktemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// use passed context with timeout
	dCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// run yt-dlp. use a generic name "clip" so we don't have to guess the title.
	outTpl := filepath.Join(tmpDir, "clip.%(ext)s")

	cmd := exec.CommandContext(
		dCtx, "yt-dlp",
		"-f", "bestvideo+bestaudio/best",
		"--no-playlist",
		"-q", "--no-warnings", "--no-progress",
		"-o", outTpl,
		rawURL,
	)

	// capture stderr in case of failure
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp failed: %v\n%s", err, strings.TrimSpace(stderr.String()))
	}

	// find the file, should be the only file in tmpDir
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", fmt.Errorf("failed to read temp dir: %w", err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no files found in temp dir %s", tmpDir)
	}
	if len(entries) > 1 {
		return "", fmt.Errorf("multiple files found in temp dir %s", tmpDir)
	}

	e := entries[0]
	if e.IsDir() {
		return "", fmt.Errorf("yt-dlp ran but found a directory %s instead of a file", e.Name())
	}

	downloadedPath := filepath.Join(tmpDir, e.Name())
	if downloadedPath == "" {
		return "", fmt.Errorf("yt-dlp ran but no file was found in %s", tmpDir)
	}

	// move to a safe location outside tmpDir
	finalFileName := fmt.Sprintf("yt-final-%d-%s", time.Now().UnixNano(), filepath.Base(downloadedPath))
	finalPath := filepath.Join(os.TempDir(), finalFileName)
	if err := os.Rename(downloadedPath, finalPath); err != nil {
		return "", fmt.Errorf("failed to move file to final destination: %w", err)
	}

	return finalPath, nil
}

// YtDLPLength probes the length of a YouTube video in seconds using yt-dlp.
func YtDLPLength(ctx context.Context, rawURL string, timeout time.Duration) (int, error) {
	if err := ensureTool("yt-dlp"); err != nil {
		return 0, err
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	pCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := exec.CommandContext(
		pCtx, "yt-dlp",
		"-q", "--no-warnings", "--no-progress",
		"--print", "duration",
		rawURL,
	)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		// if context timed out, surface that explicitly.
		if errors.Is(pCtx.Err(), context.DeadlineExceeded) {
			return 0, fmt.Errorf("yt-dlp duration probe timed out: %s", strings.TrimSpace(stderr.String()))
		}
		return 0, fmt.Errorf("yt-dlp duration probe failed: %v\n%s", err, strings.TrimSpace(stderr.String()))
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return 0, fmt.Errorf("yt-dlp duration probe returned empty output; stderr:\n%s", strings.TrimSpace(stderr.String()))
	}

	// use the last non-empty line, in case yt-dlp ever prints extra stuff.
	var last string
	for _, ln := range strings.Split(out, "\n") {
		s := strings.TrimSpace(ln)
		if s != "" {
			last = s
		}
	}
	if last == "" {
		return 0, fmt.Errorf("yt-dlp duration probe produced no usable duration; raw:\n%s", out)
	}

	// yt-dlp usually prints an integer number of seconds, but be tolerant of floats.
	secFloat, err := strconv.ParseFloat(last, 64)
	if err != nil {
		// yt-dlp can also print "NA" for live/no-duration cases.
		lower := strings.ToLower(last)
		if lower == "na" || lower == "none" {
			return 0, fmt.Errorf("yt-dlp duration unavailable (live or missing); raw:%q", last)
		}
		return 0, fmt.Errorf("yt-dlp duration parse failed for %q: %w", last, err)
	}

	if secFloat < 0 {
		return 0, fmt.Errorf("yt-dlp returned negative duration %f", secFloat)
	}

	// truncate toward zero; yt-dlp should give exact seconds anyway.
	return int(secFloat), nil
}

// ---- internal ----

type downloadFn func(ctx context.Context, rawURL, outPath, userAgent string) error

func fetchWithTempFile(ctx context.Context, plan DownloadPlan, tool, userAgent string, runner downloadFn) (string, error) {
	if plan.OutputExt == "" {
		return "", fmt.Errorf("plan output extension missing for %s", plan.Strategy)
	}

	if err := ensureTool(tool); err != nil {
		return "", err
	}

	outFile, err := os.CreateTemp("", "*."+plan.OutputExt)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	outPath := outFile.Name()
	_ = outFile.Close()

	if err := runner(ctx, plan.URL, outPath, userAgent); err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	return outPath, nil
}

func ensureTool(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s not found in PATH: %w", name, err)
	}
	return nil
}

func runFFmpeg(ctx context.Context, rawURL, outPath, userAgent string) error {
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-y",
		"-user_agent", userAgent,
		"-allowed_extensions", "ALL",
		"-i", rawURL,
		"-c", "copy",
		outPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// include last line of ffmpeg output when possible
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		// If context timed out, surface that explicitly.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("ffmpeg timed out: %s", msg)
		}
		// Map HTTP 429-ish errors to a stable package-level sentinel.
		if isTooManyRequestsMessage(msg) {
			return ErrTooManyRequests
		}
		return fmt.Errorf("ffmpeg failed: %s", msg)
	}
	return nil
}

func runCurl(ctx context.Context, rawURL, outPath, userAgent string) error {
	cmd := exec.CommandContext(
		ctx,
		"curl",
		"-fsSL",
		"-A", userAgent,
		"-w", "%{http_code}",
		"-o", outPath,
		rawURL,
	)
	out, err := cmd.CombinedOutput()
	msg := strings.TrimSpace(string(out))
	if err != nil {
		if msg == "" {
			msg = err.Error()
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("curl timed out: %s", msg)
		}
		// Check the HTTP status code from -w "%{http_code}".
		if status := parseCurlStatusCode(msg); status == 429 {
			return ErrTooManyRequests
		}
		// Fallback to message sniffing in case status parse fails but server
		// text still mentions rate limiting.
		if isTooManyRequestsMessage(msg) {
			return ErrTooManyRequests
		}
		return fmt.Errorf("curl failed: %s", msg)
	}
	return nil
}

// isTooManyRequestsMessage does a best-effort sniff for HTTP 429 / rate limit messages.
func isTooManyRequestsMessage(msg string) bool {
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "too many requests") {
		return true
	}
	// Fallback: look for a bare 429 in the text.
	if strings.Contains(msg, "429") {
		return true
	}
	return false
}

// parseCurlStatusCode tries to pull an HTTP status code out of curl's -w "%{http_code}" output.
func parseCurlStatusCode(msg string) int {
	fields := strings.Fields(msg)
	for i := len(fields) - 1; i >= 0; i-- {
		if len(fields[i]) != 3 {
			continue
		}
		n, err := strconv.Atoi(fields[i])
		if err == nil {
			return n
		}
	}
	return 0
}
