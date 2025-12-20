package externallinks

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sprout/internal/app"
	"sprout/internal/platform/database"
	"sprout/internal/platform/download"
	"sprout/pkg/xcrypto"
	"strings"

	"github.com/disgoorg/disgo/discord"
)

var AntiRotDomains = []download.Domain{
	download.DomainReddit,
	download.DomainYouTube,
	download.DomainYoutubeShorts,
	download.DomainRedGifs,
}

type Link struct {
	Url         string
	Domain      download.Domain
	CrossSrcUrl string
}

func ExtractLinks(message *discord.Message) []Link {
	// extract anti-rot supported links into slice.
	fields := strings.Fields(message.Content)
	links := make([]Link, 0)
	for _, field := range fields {
		if download.IsSingleValidURL(field) {
			domain := download.ParseDomain(field)
			if slices.Contains(AntiRotDomains, domain) {
				links = append(links, Link{Url: field, Domain: domain})
			}
		}
	}
	return links
}

func ExtractLinksFromButtons(message *discord.Message) []Link {
	links := make([]Link, 0)
	for _, c := range message.Components {
		discordActionRow, ok := c.(discord.ActionRowComponent)
		if !ok {
			continue
		}
		for _, comp := range discordActionRow.Components {
			if linkButton, ok := comp.(discord.ButtonComponent); ok {
				if (linkButton.Style == discord.ButtonStyleLink) && ((linkButton.Emoji != nil) || (linkButton.Label == "▲")) {
					links = append(links, Link{Url: linkButton.URL, Domain: download.ParseDomain(linkButton.URL)})
				}
			}
		}
		break
	}
	return links
}

// AddAsset is a helper for adding downloaded temp files to the database / assets directory.
func AddAsset(a *app.App, url, path string) error {
	// hash
	hash, err := xcrypto.FileSHA256(path)
	if err != nil {
		return fmt.Errorf("failed to hash: %w", err)
	}
	assetName := fmt.Sprintf("%s%s", hash, filepath.Ext(path))
	finalPath := database.ToAssetPath(filepath.Join(a.StorageDir, "assets"), assetName)

	// move
	// create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if err := os.Rename(path, finalPath); err != nil {
		return fmt.Errorf("failed to move: %w", err)
	}

	// upsert
	if _, err := database.UpsertAsset(a.DB, url, func(asset *database.Asset) error {
		asset.Path = finalPath
		return nil
	}); err != nil {
		return fmt.Errorf("failed to upsert asset: %w", err)
	}
	return nil
}
