package compressor

import "strings"

// MediaType describes the type of media file based on extension.
type MediaType string

const (
	MediaTypeVideo   MediaType = "video"
	MediaTypeImage   MediaType = "image"
	MediaTypeUnknown MediaType = "unknown"
)

// Common file extensions mapped to media types.
var videoExtensions = map[string]struct{}{
	"mp4":  {},
	"mkv":  {},
	"webm": {},
	"avi":  {},
	"mov":  {},
	"wmv":  {},
	"flv":  {},
	"m4v":  {},
	"mpeg": {},
	"mpg":  {},
	"3gp":  {},
	"ts":   {},
	"mts":  {},
	"m2ts": {},
	"vob":  {},
	"ogv":  {},
	"rm":   {},
	"rmvb": {},
	"asf":  {},
	"divx": {},
	"f4v":  {},
}

var imageExtensions = map[string]struct{}{
	"jpg":  {},
	"jpeg": {},
	"png":  {},
	"gif":  {},
	"webp": {},
	"bmp":  {},
	"tiff": {},
	"tif":  {},
	"svg":  {},
	"ico":  {},
	"heic": {},
	"heif": {},
	"avif": {},
	"raw":  {},
	"cr2":  {},
	"nef":  {},
	"orf":  {},
	"sr2":  {},
	"psd":  {},
	"ai":   {},
	"eps":  {},
	"jfif": {},
}

// MediaTypeFromExt returns the media type for a given file extension.
// The extension can be provided with or without a leading dot (e.g., "mp4" or ".mp4").
func MediaTypeFromExt(ext string) MediaType {
	// Normalize: remove leading dot and lowercase
	ext = strings.TrimPrefix(strings.ToLower(ext), ".")

	if _, ok := videoExtensions[ext]; ok {
		return MediaTypeVideo
	}
	if _, ok := imageExtensions[ext]; ok {
		return MediaTypeImage
	}
	return MediaTypeUnknown
}
