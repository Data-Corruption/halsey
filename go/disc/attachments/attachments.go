package attachments

import (
	"path/filepath"
	"strings"

	"github.com/disgoorg/disgo/discord"
)

func IsMedia(att discord.Attachment) bool {
	if att.ContentType != nil {
		if strings.HasPrefix(*att.ContentType, "image/") || strings.HasPrefix(*att.ContentType, "video/") {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(att.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".mp4", ".mov", ".webm":
		return true
	default:
		return false
	}
}
