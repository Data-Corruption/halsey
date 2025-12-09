package emojis

import (
	"fmt"
	"math/rand"
	"sprout/internal/app"
	"strings"
	"sync"
)

var (
	emojiCache = []string{}
	fillCache  sync.Once
)

// GetAppEmojis retrieves the application emojis.
// It caches the emojis to avoid repeated API calls.
// Strips the < and > from the emoji ID if they exist.
func GetAppEmojis(a *app.App) []string {
	fillCache.Do(func() {
		emojis, err := a.Client.Rest.GetApplicationEmojis(a.Client.ApplicationID)
		if err != nil {
			a.Log.Errorf("failed to get emojis: %v\n", err)
			return
		}
		for _, emoji := range emojis {
			strID := emoji.String()
			if strID[0] == '<' && strID[len(strID)-1] == '>' {
				strID = strID[1 : len(strID)-1]
			}
			emojiCache = append(emojiCache, strID)
		}
	})
	return emojiCache
}

func GetSpinnerEmoji(a *app.App) string {
	emojis := GetAppEmojis(a)
	for _, emoji := range emojis {
		if strings.HasPrefix(emoji, "a:spinner") {
			return "<" + emoji + ">"
		}
	}
	return ""
}

func GetRandFavEmoji(a *app.App) (string, error) {
	emojis := GetAppEmojis(a)
	if len(emojis) == 0 {
		return "", fmt.Errorf("no emojis found")
	}
	var favoriteEmojis = []string{}
	for _, emoji := range emojis {
		if strings.HasPrefix(emoji, "a:fav") || strings.HasPrefix(emoji, ":fav") {
			favoriteEmojis = append(favoriteEmojis, emoji)
		}
	}
	return favoriteEmojis[rand.Intn(len(favoriteEmojis))], nil
}
