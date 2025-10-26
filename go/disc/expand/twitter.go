package expand

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/storage/config"
	"strings"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
)

func twitter(ctx context.Context, sourceMessage *discord.Message, url string) error {
	// load state from config
	enabled, err := config.Get[bool](ctx, "expandTwitter")
	if err != nil {
		return fmt.Errorf("failed to get expandTwitter from config: %w", err)
	}
	if !enabled {
		xlog.Debugf(ctx, "Twitter expansion is disabled, skipping")
		return nil
	}

	// change prefix from https://x.com/ to https://fxtwitter.com/
	mUrl := strings.Replace(url, "https://x.com/", "https://g.fxtwitter.com/", 1)

	// create a message with the expanded URL
	_, err = disc.Client.Rest.CreateMessage(sourceMessage.ChannelID, discord.NewMessageCreateBuilder().
		AddActionRow(discord.NewLinkButton("Open In Twitter", url)).
		SetContentf("%s\n`%s` • <t:%d:f>", mUrl, sourceMessage.Author.Username, sourceMessage.CreatedAt.Unix()).
		Build())
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// delete the original message
	if err := disc.Client.Rest.DeleteMessage(sourceMessage.ChannelID, sourceMessage.ID); err != nil {
		return fmt.Errorf("failed to delete original message: %w", err)
	}

	return nil
}
