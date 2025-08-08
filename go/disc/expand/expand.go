package expand

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/xnet"
	"net/url"
	"strings"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
)

func Expand(ctx context.Context, sourceMessage *discord.Message, manual bool) error {
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
		if manual && xnet.IsYouTubeURL(url) {
			return youtube(ctx, sourceMessage, url)
		} else if xnet.IsYouTubeShortsURL(url) {
			return youtube(ctx, sourceMessage, url)
		}
		return nil
	}
}

// ---- internal ----

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

// ---- status message helpers ----

func createStatusMessage(ctx context.Context, sourceMessage *discord.Message, content string) (*discord.Message, error) {
	return disc.Client.Rest.CreateMessage(sourceMessage.ChannelID, discord.NewMessageCreateBuilder().
		SetMessageReferenceByID(sourceMessage.ID).
		SetContent(disc.GetSpinnerEmoji(ctx)+" "+content).
		Build())
}

func updateStatusMessage(ctx context.Context, msg *discord.Message, content string) error {
	if _, err := disc.Client.Rest.UpdateMessage(msg.ChannelID, msg.ID, discord.NewMessageUpdateBuilder().
		SetContent(disc.GetSpinnerEmoji(ctx)+" "+content).
		Build()); err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}
	return nil
}

// updates the status message and deletes it after a delay if given
func failStatusMessage(ctx context.Context, msg *discord.Message, content string, d time.Duration) error {
	if _, err := disc.Client.Rest.UpdateMessage(msg.ChannelID, msg.ID, discord.NewMessageUpdateBuilder().
		SetContent("🔴 "+content).
		Build()); err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}
	if d > 0 {
		time.AfterFunc(d, func() {
			if err := disc.Client.Rest.DeleteMessage(msg.ChannelID, msg.ID); err != nil {
				xlog.Errorf(ctx, "failed to delete status message: %s", err)
			}
		})
	}
	return nil
}
