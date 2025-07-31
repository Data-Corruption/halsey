package expand

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"net/url"
	"strings"

	"github.com/disgoorg/disgo/discord"
)

func ExpandTest(ctx context.Context, sourceMessage *discord.Message) error {
	url := strings.TrimSpace(sourceMessage.Content)
	if !isSingleValidURL(url) {
		return nil
	}

	// {"https://www.reddit.com", "https://www.old.reddit.com"}

	switch {
	case strings.HasPrefix(url, "https://x.com/"):
		return twitter(ctx, sourceMessage, url)
	case strings.HasPrefix(url, "https://www.instagram.com/"):
		return instagram(ctx, sourceMessage, url)
	case strings.HasPrefix(url, "https://www.reddit.com/") || strings.HasPrefix(url, "https://old.reddit.com/"):
		return reddit(ctx, sourceMessage, url)
	default:
		// fallback
	}

	/*
		fmt.Println("Expanding message:", sourceMessage.ID)

		sp := disc.GetSpinnerEmoji(ctx)

		fMsg, err := disc.Client.Rest.CreateMessage(sourceMessage.ChannelID, discord.NewMessageCreateBuilder().
			SetMessageReferenceByID(sourceMessage.ID).
			SetContent(sp+" Expanding message... ").
			Build())
		if err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}

		time.Sleep(4 * time.Second)

		if _, err = disc.Client.Rest.UpdateMessage(sourceMessage.ChannelID, fMsg.ID, discord.NewMessageUpdateBuilder().
			SetContent(sp+" Updated message... ").
			Build()); err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}

		time.Sleep(4 * time.Second)

		if _, err = disc.Client.Rest.UpdateMessage(sourceMessage.ChannelID, fMsg.ID, discord.NewMessageUpdateBuilder().
			SetContent(sp+" Finalizing message... ").
			Build()); err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}

		time.Sleep(4 * time.Second)

		// delete the message
		if err := disc.Client.Rest.DeleteMessage(sourceMessage.ChannelID, fMsg.ID); err != nil {
			return err
		}
	*/

	return nil
}

func updateStatusMessage(ctx context.Context, msg *discord.Message, content string) error {
	sp := disc.GetSpinnerEmoji(ctx)
	if _, err := disc.Client.Rest.UpdateMessage(msg.ChannelID, msg.ID, discord.NewMessageUpdateBuilder().
		SetContent(sp+" "+content).
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
