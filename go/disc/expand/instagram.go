package expand

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"strings"

	"github.com/disgoorg/disgo/discord"
)

func instagram(ctx context.Context, sourceMessage *discord.Message, url string) error {
	// change prefix from https://www.instagram.com/ to https://www.ddinstagram.com/
	mUrl := strings.Replace(url, "https://www.instagram.com/", "https://g.ddinstagram.com/", 1)

	// create a message with the expanded URL
	_, err := disc.Client.Rest.CreateMessage(sourceMessage.ChannelID, discord.NewMessageCreateBuilder().
		SetContentf("%s\n`%s` • <t:%d:R>", mUrl, sourceMessage.Author.Username, sourceMessage.CreatedAt.Unix()).
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
