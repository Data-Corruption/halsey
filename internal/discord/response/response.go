package response

import (
	"fmt"
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// Emoji is the unicode emoji or custom emoji ID e.g.
// a:name:1399243822592163930 (animated) or name:1399243822592163930 (static).
// To get that ID, for uploaded bot emojis, you can copy it from where you uploaded it.
// For the rest you can message a channel with \:name: and it will return the full ID
// in the message. Why can't you right click and copy ID like everything else? I don't
// know, discord is weird. This glorified wrapper is mostly just for this comment lmao
func ReactToMessage(a *app.App, channelID snowflake.ID, messageID snowflake.ID, emoji string) error {
	return a.Client.Rest.AddReaction(channelID, messageID, emoji)
}

// MessageBotChannel sends a message to the bot channel defined in config.
// Returns the created message or an error.
// messageCreate example: discord.NewMessageCreateBuilder().SetContent("Hello, bot channel!").Build()
func MessageBotChannel(a *app.App, messageCreate discord.MessageCreate) (*discord.Message, error) {
	cfg, err := database.ViewConfig(a.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to view config: %w", err)
	}
	if cfg.BotChannelID == 0 {
		return nil, fmt.Errorf("bot channel ID is not set in general settings")
	}
	return a.Client.Rest.CreateMessage(cfg.BotChannelID, messageCreate)
}
