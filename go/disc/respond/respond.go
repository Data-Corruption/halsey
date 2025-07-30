package respond

import (
	"context"
	"halsey/go/disc"
	"halsey/go/storage/config"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// Emoji is the unicode emoji or custom emoji ID e.g.
// a:name:1399243822592163930 (animated) or name:1399243822592163930 (static).
// To get that ID, for uploaded bot emojis, you can copy it from where you uploaded it.
// For the rest you can message a channel with \:name: and it will return the full ID
// in the message. Why can't you right click and copy ID like everything else? I don't
// know, discord is weird. This glorified wrapper is mostly just for this comment lmao
func React(client *bot.Client, channelID snowflake.ID, messageID snowflake.ID, emoji string) error {
	return client.Rest.AddReaction(channelID, messageID, emoji)
}

func BotChannel(ctx context.Context, msg string) {
	// get bot channel ID from config
	id, err := config.Get[string](ctx, "botChannelID")
	if err != nil {
		xlog.Errorf(ctx, "Error getting bot channel ID from config: %s", err)
		return
	}
	if id == "" {
		xlog.Error(ctx, "Bot channel ID not set in config")
		return
	}

	botChannelID, err := snowflake.Parse(id)
	if err != nil {
		xlog.Errorf(ctx, "Error parsing bot channel ID: %s", err)
		return
	}
	if _, err := disc.Client.Rest.CreateMessage(botChannelID, discord.NewMessageCreateBuilder().SetContent(msg).Build()); err != nil {
		xlog.Errorf(ctx, "Error sending message to bot channel: %s", err)
	}
}
