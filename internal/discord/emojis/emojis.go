package emojis

import (
	"math/rand"
	"sprout/internal/app"
	"sprout/internal/platform/download"
	"strings"
	"sync"

	"github.com/disgoorg/disgo/discord"
)

const (
	SpinnerPrefix = "spinner"
	FavPrefix     = "fav"

	// social media prefixes
	RedditPrefix  = "reddit"
	YtPrefix      = "youtube"
	RedGifsPrefix = "18_plus"
	UnknownPrefix = "unknown"
)

var (
	emojiCache = []discord.Emoji{}
	fillCache  sync.Once
)

// GetAppEmojis retrieves the application emojis.
// It caches the emojis to avoid repeated API calls.
// Strips the < and > from the emoji ID if they exist.
func GetAppEmojis(a *app.App) []discord.Emoji {
	fillCache.Do(func() {
		var err error
		emojiCache, err = a.Client.Rest.GetApplicationEmojis(a.Client.ApplicationID)
		if err != nil {
			a.Log.Errorf("failed to get emojis: %v\n", err)
		}
	})
	return emojiCache
}

func GetSpinnerEmoji(a *app.App) (discord.Emoji, bool) {
	return get(a, SpinnerPrefix)
}

func GetRandFavEmoji(a *app.App) (discord.Emoji, bool) {
	emojis := GetAppEmojis(a)
	var favoriteEmojis = []discord.Emoji{}
	for _, emoji := range emojis {
		if strings.HasPrefix(emoji.String(), "<a:"+FavPrefix) || strings.HasPrefix(emoji.String(), "<:"+FavPrefix) {
			favoriteEmojis = append(favoriteEmojis, emoji)
		}
	}
	if len(favoriteEmojis) == 0 {
		return discord.Emoji{}, false
	}
	return favoriteEmojis[rand.Intn(len(favoriteEmojis))], true
}

func GetSocialMediaEmoji(a *app.App, domain download.Domain) (discord.Emoji, bool) {
	switch domain {
	case download.DomainReddit:
		return get(a, RedditPrefix)
	case download.DomainYouTube, download.DomainYoutubeShorts:
		return get(a, YtPrefix)
	case download.DomainRedGifs:
		return get(a, RedGifsPrefix)
	default:
		return get(a, UnknownPrefix)
	}
}

func get(a *app.App, prefix string) (discord.Emoji, bool) {
	emojis := GetAppEmojis(a)
	for _, emoji := range emojis {
		if strings.HasPrefix(emoji.String(), "<a:"+prefix) || strings.HasPrefix(emoji.String(), "<:"+prefix) {
			return emoji, true
		}
	}
	return discord.Emoji{}, false
}
