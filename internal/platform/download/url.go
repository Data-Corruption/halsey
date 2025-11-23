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
	StrategyFFmpeg
	StrategyDirect
)

func (s Strategy) String() string {
	switch s {
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
	case p.OutputExt == "":
		return errors.New("plan output extension is empty")
	default:
		return nil
	}
}

// ParseMediaURL analyzes a media URL and returns the download plan.
func ParseMediaURL(rawURL string) (DownloadPlan, error) {
	if rawURL == "" {
		return DownloadPlan{}, fmt.Errorf("invalid url: empty")
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return DownloadPlan{}, fmt.Errorf("invalid url: %w", err)
	}

	plan := DownloadPlan{
		URL: rawURL,
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
		plan.OutputExt = "mp4" // force mp4 out (for discord inline player compatibility)
	case "mp4":
		plan.Strategy = StrategyFFmpeg
	default:
		plan.Strategy = StrategyDirect
	}

	return plan, nil
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
