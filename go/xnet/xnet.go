package xnet

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Ffmpeg runs the ffmpeg command to download media from the given URL and save it to the specified output path.
// It returns an error if the ffmpeg command fails.
func Ffmpeg(url string, outPath string) error {
	cmd := exec.Command("ffmpeg", "-i", url, "-c", "copy", outPath)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg command failed: %w", err)
	}
	return nil
}

func parseTimestampToSeconds(ts string) (float64, error) {
	parts := strings.Split(ts, ":")
	if len(parts) != 3 {
		return 0, errors.New("invalid timestamp format")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	s, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, err
	}
	return float64(h)*3600 + float64(m)*60 + s, nil
}

func getMediaDuration(url string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json",
		"-show_format", "-show_streams", url)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to run ffprobe: %w", err)
	}

	var ffprobeData struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(out, &ffprobeData); err != nil {
		return 0, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}
	if ffprobeData.Format.Duration == "" {
		return 0, errors.New("no duration found in ffprobe output")
	}

	dur, err := strconv.ParseFloat(ffprobeData.Format.Duration, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %w", err)
	}
	return dur, nil
}

// Curl runs the curl command to download a file from the given URL and save it to the specified output path.
// It returns an error if the curl command fails.
func Curl(url string, outPath string) error {
	cmd := exec.Command("curl", "-o", outPath, url)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("curl command failed: %w", err)
	}
	return nil
}

func DownloadMedia(url string) (string, error) {
	var ext string
	var err error
	// determine the file extension
	if ext, err = extractFileType(url); err != nil {
		return "", err
	}
	// generate a temporary file, serves as a reservation
	reservePath, err := GenTempFile()
	if err != nil {
		return "", err
	}
	defer RemoveFile(reservePath)                      // remove the reservation file
	if slices.Contains([]string{"m3u8", "mpd"}, ext) { // change video formats to mp4
		ext = "mp4"
	}
	outPath := reservePath + "." + ext
	if slices.Contains([]string{"mp4"}, ext) { // add other formats as needed
		return outPath, Ffmpeg(url, outPath)
	} else {
		return outPath, Curl(url, outPath)
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
