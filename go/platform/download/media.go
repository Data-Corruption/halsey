package download

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Strategy represents the toolchain needed to fetch the media.
type Strategy int

const (
	StrategyUnknown Strategy = iota
	StrategyYouTube
	StrategyFFmpeg
	StrategyDirect
)

func (s Strategy) String() string {
	switch s {
	case StrategyYouTube:
		return "yt-dlp"
	case StrategyFFmpeg:
		return "ffmpeg"
	case StrategyDirect:
		return "direct"
	default:
		return "unknown"
	}
}

// DownloadPlan captures how to retrieve a remote media resource.
type DownloadPlan struct {
	URL       string
	Ext       string
	OutputExt string
	Strategy  Strategy
}

// Validate ensures the plan has enough information to be executed.
func (p DownloadPlan) Validate() error {
	switch {
	case p.URL == "":
		return errors.New("plan URL is empty")
	case p.Strategy == StrategyUnknown:
		return errors.New("plan strategy is unknown")
	case p.Strategy != StrategyYouTube && p.OutputExt == "":
		return errors.New("plan output extension is empty")
	default:
		return nil
	}
}

// Parse analyzes a media URL and returns the download plan.
func Parse(rawURL string) (DownloadPlan, error) {
	if rawURL == "" {
		return DownloadPlan{}, fmt.Errorf("invalid url: empty")
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return DownloadPlan{}, fmt.Errorf("invalid url: %w", err)
	}

	plan := DownloadPlan{
		URL: rawURL,
	}

	if IsYouTubeURL(rawURL) {
		plan.Strategy = StrategyYouTube
		return plan, nil
	}

	ext, err := extractFileType(rawURL)
	if err != nil {
		return DownloadPlan{}, err
	}
	plan.Ext = ext
	plan.OutputExt = ext

	switch ext {
	case "m3u8", "mpd":
		plan.Strategy = StrategyFFmpeg
		plan.OutputExt = "mp4"
	case "mp4":
		plan.Strategy = StrategyFFmpeg
	default:
		plan.Strategy = StrategyDirect
	}

	return plan, nil
}

// IsYouTubeURL returns true when the URL points at a YouTube resource.
func IsYouTubeURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "https://youtu.be/") ||
		strings.HasPrefix(rawURL, "https://www.youtube.com/watch?v=") ||
		IsYouTubeShortsURL(rawURL)
}

// IsYouTubeShortsURL returns true when the URL points at a YouTube Shorts resource.
func IsYouTubeShortsURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "https://youtube.com/shorts/") ||
		strings.HasPrefix(rawURL, "https://www.youtube.com/shorts/")
}

var extRegex = regexp.MustCompile(`\.([a-zA-Z0-9]+)(?:[?#]|$)`)

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
