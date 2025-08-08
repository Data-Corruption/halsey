package xnet

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func IsYouTubeURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "https://youtu.be/") || strings.HasPrefix(rawURL, "https://www.youtube.com/watch?v=") || IsYouTubeShortsURL(rawURL)
}
func IsYouTubeShortsURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "https://youtube.com/shorts/") || strings.HasPrefix(rawURL, "https://www.youtube.com/shorts/")
}

// DownloadMedia downloads media from the given URL and saves it to a temporary file.
// It returns the output file path. Caller is responsible for cleanup of that file.
func DownloadMedia(rawURL string, timeout time.Duration) (string, error) {
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if IsYouTubeURL(rawURL) {
		return ytDLP(ctx, rawURL)
	}

	ext, err := extractFileType(rawURL)
	if err != nil {
		return "", err
	}

	// Decide output extension & tool
	useFFmpeg := false
	outExt := ext
	switch {
	case ext == "m3u8" || ext == "mpd": // containerized streaming → remux to mp4
		useFFmpeg = true
		outExt = "mp4"
	case ext == "mp4":
		useFFmpeg = true
	default:
		useFFmpeg = false
	}

	outFile, err := os.CreateTemp("", "*."+outExt)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	outPath := outFile.Name()
	_ = outFile.Close() // allow external tools to write

	if useFFmpeg {
		if err := ensureTool("ffmpeg"); err != nil {
			return "", err
		}
		if err := runFFmpeg(ctx, rawURL, outPath); err != nil {
			_ = os.Remove(outPath)
			return "", err
		}
		return outPath, nil
	}

	if err := ensureTool("curl"); err != nil {
		return "", err
	}
	if err := runCurl(ctx, rawURL, outPath); err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	return outPath, nil
}

// ---- internal ----

var extRegex = regexp.MustCompile(`\.([a-zA-Z0-9]+)(?:[?#]|$)`)

func ensureTool(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s not found in PATH: %w", name, err)
	}
	return nil
}

func runFFmpeg(ctx context.Context, rawURL, outPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", rawURL, "-c", "copy", outPath)
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
	cmd := exec.CommandContext(ctx, "curl", "-fsSL", "-o", outPath, rawURL)
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

	format := `bestvideo[height<=720]+bestaudio/best[height<=720]`

	outTpl := filepath.Join(tmpDir, "clip.%(ext)s")
	cmd := exec.CommandContext(
		ctx, "yt-dlp",
		"-f", format,
		"--no-playlist",
		"--force-overwrites",
		"-q", "--no-warnings",
		"--print", "after_move:filepath",
		"-o", outTpl,
		rawURL,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w\n%s", err, out)
	}
	finalPath := strings.TrimSpace(string(out))
	if finalPath == "" {
		return "", fmt.Errorf("yt-dlp did not return a filepath:\n%s", out)
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
		data, rerr := os.ReadFile(finalPath)
		if rerr != nil {
			_ = os.Remove(dstPath)
			return "", fmt.Errorf("move yt-dlp output: %w (and failed read: %v)", err, rerr)
		}
		if werr := os.WriteFile(dstPath, data, 0o600); werr != nil {
			_ = os.Remove(dstPath)
			return "", fmt.Errorf("write yt-dlp output: %w", werr)
		}
	}
	return dstPath, nil
}

// extractFileType returns the URL-indicated extension or an error if none is found.
// It first checks the "format" query parameter, then the path suffix.
func extractFileType(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}

	if format := u.Query().Get("format"); format != "" {
		return format, nil
	}

	m := extRegex.FindStringSubmatch(u.Path)
	if len(m) < 2 {
		return "", fmt.Errorf("no file extension found in url: %s", rawURL)
	}
	return strings.ToLower(m[1]), nil
}
