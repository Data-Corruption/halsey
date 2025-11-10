package download

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
)

const defaultTimeout = 2 * time.Minute

// Download parses the URL into a DownloadPlan and executes it.
func Download(rawURL string, timeout time.Duration) (string, error) {
	plan, err := Parse(rawURL)
	if err != nil {
		return "", err
	}
	return DownloadWithPlan(plan, timeout)
}

// DownloadWithPlan executes a previously parsed download plan.
func DownloadWithPlan(plan DownloadPlan, timeout time.Duration) (string, error) {
	if err := plan.Validate(); err != nil {
		return "", err
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch plan.Strategy {
	case StrategyYouTube:
		return ytDLP(ctx, plan.URL)
	case StrategyFFmpeg:
		return fetchWithTempFile(ctx, plan, "ffmpeg", runFFmpeg)
	case StrategyDirect:
		return fetchWithTempFile(ctx, plan, "curl", runCurl)
	default:
		return "", fmt.Errorf("unsupported download strategy: %s", plan.Strategy)
	}
}

// ---- internal ----

type downloadFn func(ctx context.Context, rawURL, outPath string) error

func fetchWithTempFile(ctx context.Context, plan DownloadPlan, tool string, runner downloadFn) (string, error) {
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

	if err := runner(ctx, plan.URL, outPath); err != nil {
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

func runFFmpeg(ctx context.Context, rawURL, outPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-user_agent", "Mozilla/5.0", "-allowed_extensions", "ALL", "-i", rawURL, "-c", "copy", outPath)
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
		return fmt.Errorf("ffmpeg failed: %s", msg)
	}
	return nil
}

func runCurl(ctx context.Context, rawURL, outPath string) error {
	cmd := exec.CommandContext(ctx, "curl", "-fsSL", "-A", "Mozilla/5.0", "-o", outPath, rawURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("curl timed out: %s", msg)
		}
		return fmt.Errorf("curl failed: %s", msg)
	}
	return nil
}

// ytDLP downloads YouTube media via yt-dlp to a temp file and returns its path.
// no need for mutex or anything, each run gets its own temp dir
func ytDLP(ctx context.Context, rawURL string) (string, error) {
	if err := ensureTool("yt-dlp"); err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "yt-")
	if err != nil {
		return "", fmt.Errorf("mktemp: %w", err)
	}
	// Always attempt to remove the staging dir; we'll move the file out on success.
	defer func() { _ = os.RemoveAll(tmpDir) }()

	outTpl := filepath.Join(tmpDir, "clip.%(ext)s")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(
		ctx, "yt-dlp",
		"-f", `bestvideo[height<=720]+bestaudio/best[height<=720]`,
		"-N", "8",
		"--geo-bypass",
		"--no-playlist",
		"--force-overwrites",
		"-q", "--no-warnings", "--no-progress",
		"--print", "after_move:filepath",
		"-o", outTpl,
		rawURL,
	)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp failed: %v\n%s", err, strings.TrimSpace(stderr.String()))
	}

	// prefer the last non-empty stdout line that exists on disk
	var finalPath string
	for _, ln := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		s := strings.TrimSpace(ln)
		if s != "" {
			finalPath = s // will end up last
		}
	}
	if finalPath == "" || !strings.HasPrefix(finalPath, tmpDir) {
		// fallback: find the newest file in tmpDir matching clip.*
		matches, _ := filepath.Glob(filepath.Join(tmpDir, "clip.*"))
		if len(matches) > 0 {
			sort.Slice(matches, func(i, j int) bool {
				ai, _ := os.Stat(matches[i])
				aj, _ := os.Stat(matches[j])
				return ai.ModTime().After(aj.ModTime())
			})
			finalPath = matches[0]
		}
	}
	if finalPath == "" {
		return "", fmt.Errorf("yt-dlp did not return a filepath; stderr:\n%s", strings.TrimSpace(stderr.String()))
	}

	// Move the file out to a real temp file and return that path.
	dstExt := filepath.Ext(finalPath)
	dst, err := os.CreateTemp("", "clip-*"+dstExt)
	if err != nil {
		return "", fmt.Errorf("create temp for yt-dlp output: %w", err)
	}
	dstPath := dst.Name()
	_ = dst.Close()
	_ = os.Remove(dstPath) // make room for rename

	if err := os.Rename(finalPath, dstPath); err != nil {
		// Cross-device (unlikely for os.TempDir), fallback to copy
		data, rErr := os.ReadFile(finalPath)
		if rErr != nil {
			_ = os.Remove(dstPath)
			return "", fmt.Errorf("move yt-dlp output: %w (and failed read: %v)", err, rErr)
		}
		if wErr := os.WriteFile(dstPath, data, 0o600); wErr != nil {
			_ = os.Remove(dstPath)
			return "", fmt.Errorf("write yt-dlp output: %w", wErr)
		}
	}
	return dstPath, nil
}
