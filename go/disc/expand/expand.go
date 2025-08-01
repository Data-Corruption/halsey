package expand

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/xnet"
	"net/url"
	"strings"

	"github.com/disgoorg/disgo/discord"
)

func ExpandTest(ctx context.Context, sourceMessage *discord.Message, manual bool) error {
	url := strings.TrimSpace(sourceMessage.Content)
	if !isSingleValidURL(url) {
		return nil
	}

	switch {
	case strings.HasPrefix(url, "https://x.com/"):
		return twitter(ctx, sourceMessage, url)
	case strings.HasPrefix(url, "https://www.instagram.com/"):
		return instagram(ctx, sourceMessage, url)
	case strings.HasPrefix(url, "https://www.reddit.com/") || strings.HasPrefix(url, "https://old.reddit.com/"):
		return reddit(ctx, sourceMessage, url)
	default:
		if manual {
			for _, prefix := range xnet.YtPrefixes {
				if strings.HasPrefix(url, prefix) {
					return youtube(ctx, sourceMessage, url)
				}
			}
		} else {
			// just shorts
			if strings.HasPrefix(url, "https://youtube.com/shorts/") || strings.HasPrefix(url, "https://www.youtube.com/shorts/") {
				return youtube(ctx, sourceMessage, url)
			}
		}
		return nil
	}
}

// helper function to update a status message
func updateStatusMessage(ctx context.Context, msg *discord.Message, spinner bool, content string) error {
	if spinner {
		content = disc.GetSpinnerEmoji(ctx) + " " + content
	}
	if _, err := disc.Client.Rest.UpdateMessage(msg.ChannelID, msg.ID, discord.NewMessageUpdateBuilder().
		SetContent(content).
		Build()); err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}
	return nil
}

// isSingleValidURL checks if the string contains a single valid URL.
// Low resource usage, high perf, gonna run on all messages.
func isSingleValidURL(s string) bool {
	// fast path
	if !(strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")) {
		return false
	}
	// fuzzy check
	count := strings.Count(s, "http://") + strings.Count(s, "https://")
	if count != 1 || strings.ContainsAny(s, " \t\n") {
		return false
	}
	// parse URL
	u, err := url.ParseRequestURI(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}
