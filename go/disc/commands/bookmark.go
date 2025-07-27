package commands

import (
	"context"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

/*
func getBookmarkChannel(guild_id string) (discord.Channel, error) {
	// get the guild's config
	guildConfig, exists := utils.Config.Guilds[guild_id]
	if !exists {
		return nil, fmt.Errorf("this guild is not configured")
	}
	// get the bookmark channel
	if guildConfig.BookmarkChannelID == "" {
		return nil, fmt.Errorf("this guild does not have an bookmark channel set")
	}
	bookmarkChannelID, err := snowflake.Parse(guildConfig.BookmarkChannelID)
	if err != nil {
		blog.Error(fmt.Sprintf("Error parsing bookmark channel ID: %s", err))
		return nil, fmt.Errorf("an error occurred")
	}
	// get the channel from the cache
	bookmarkChannel, ok := client.Client.Caches().Channel(bookmarkChannelID)
	if !ok {
		blog.Error(fmt.Sprintf("Error getting bookmark channel: %s", err))
		return nil, fmt.Errorf("an error occurred")
	}
	return bookmarkChannel, nil
}

func buildAuthorEmbed(message discord.Message, channelName string) discord.Embed {
	messageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s",
		message.GuildID.String(),
		message.ChannelID.String(),
		message.ID.String(),
	)
	footer := discord.EmbedFooter{
		Text:    message.Author.Username + " in #" + channelName,
		IconURL: *message.Author.AvatarURL(),
	}
	return discord.Embed{
		Title:     "Jump to message",
		URL:       messageLink,
		Footer:    &footer,
		Timestamp: &message.CreatedAt,
	}
}

func buildContentString(message discord.Message) string {
	content := message.Content + " "
	// add any attachments
	for _, attachment := range message.Attachments {
		content += attachment.URL + " "
	}
	return content
}
*/

var bookmarkCommand = BotCommand{
	IsGlobal: false,
	Data: discord.MessageCommandCreate{
		Name: "bookmark",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) {
		/*
			data := event.MessageCommandInteractionData()
			message := data.TargetMessage()

			bookmarkChannel, err := getBookmarkChannel(event.GuildID().String())
			if err != nil {
				utils.RespondTemp(event, "There was an error getting the bookmark channel", true, 5*time.Second)
				blog.Debug(fmt.Sprintf("Error getting bookmark channel: %s", err))
				return
			}

			// separate content into its own message so any attachments are sent as embeds

			// send content
			client.Client.Rest().CreateMessage(bookmarkChannel.ID(), discord.NewMessageCreateBuilder().
				SetContent(buildContentString(message)).
				Build(),
			)
			time.Sleep(25 * time.Millisecond)
			// send embed
			client.Client.Rest().CreateMessage(bookmarkChannel.ID(), discord.NewMessageCreateBuilder().
				SetEmbeds(buildAuthorEmbed(message, event.Channel().Name())).
				Build(),
			)

			utils.RespondTemp(event, "Bookmarked!", true, 3*time.Second)
		*/
	},
}

func init() {
	List["bookmark"] = bookmarkCommand
}
