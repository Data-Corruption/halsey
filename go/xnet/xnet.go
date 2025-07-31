package xnet

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"slices"
)

// Ffmpeg runs the ffmpeg command to download media from the given URL and save it to the specified output path.
// It returns an error if the ffmpeg command fails.
func Ffmpeg(rawURL string, outPath string) error {
	cmd := exec.Command("ffmpeg", "-y", "-i", rawURL, "-c", "copy", outPath)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg command failed: %w", err)
	}
	return nil
}

// Curl runs the curl command to download a file from the given URL and save it to the specified output path.
// It returns an error if the curl command fails.
func Curl(rawURL string, outPath string) error {
	cmd := exec.Command("curl", "-o", outPath, rawURL)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("curl command failed: %w", err)
	}
	return nil
}

// DownloadMedia downloads media from the given URL and saves it to a temporary file.
// It determines the file type from the URL and uses either ffmpeg or curl to download it.
// It returns the path to the downloaded file in a temporary directory and an error if any.
// NOTE: cleanup of outfile is the caller's responsibility.
func DownloadMedia(rawURL string) (string, error) {
	var ext string
	var err error
	// determine the file extension
	if ext, err = extractFileType(rawURL); err != nil {
		return "", err
	}
	if slices.Contains([]string{"m3u8", "mpd"}, ext) { // change video formats to mp4
		ext = "mp4"
	}
	// gen out file
	outFile, err := os.CreateTemp("", "*."+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	outFile.Close() // close so ffmpeg/curl can write to it
	// do the download
	outPath := outFile.Name()
	if slices.Contains([]string{"mp4"}, ext) { // add other formats as needed
		return outPath, Ffmpeg(rawURL, outPath)
	} else {
		return outPath, Curl(rawURL, outPath)
	}
}

func extractFileType(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	// Check for a "format" parameter in the query string
	queryParams := parsedURL.Query()
	if format := queryParams.Get("format"); format != "" {
		return format, nil
	}
	// Check for alphanumeric characters after the last period followed by a query string, hash, or end of string
	re := regexp.MustCompile(`\.([a-zA-Z0-9]+)(?:[\?#]|$)`)
	matches := re.FindStringSubmatch(parsedURL.Path)
	if len(matches) < 2 {
		return "", fmt.Errorf("no file extension found in URL: %s", rawURL)
	}
	return matches[1], nil
}
