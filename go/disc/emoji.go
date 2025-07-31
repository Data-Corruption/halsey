package disc

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/Data-Corruption/stdx/xlog"
)

var (
	emojiCache = []string{}
	fillCache  sync.Once
)

// GetAppEmojis retrieves the application emojis.
// It caches the emojis to avoid repeated API calls.
// Strips the < and > from the emoji ID if they exist.
func GetAppEmojis(ctx context.Context) []string {
	fillCache.Do(func() {
		emojis, err := Client.Rest.GetApplicationEmojis(Client.ApplicationID)
		if err != nil {
			xlog.Errorf(ctx, "failed to get emojis: %v\n", err)
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

func GetSpinnerEmoji(ctx context.Context) string {
	emojis := GetAppEmojis(ctx)
	for _, emoji := range emojis {
		if strings.HasPrefix(emoji, "a:spinner") {
			return "<" + emoji + ">"
		}
	}
	return ""
}

func GetRandFavEmoji(ctx context.Context) (string, error) {
	emojis := GetAppEmojis(ctx)
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
