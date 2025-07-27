package commands

import (
	"context"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var syncCommand = BotCommand{
	IsGlobal: false,
	Data: discord.SlashCommandCreate{
		Name:        "sync",
		Description: "Links the guild's synctube.",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) {
		/*
			urlKey := event.GuildID().String()+".synctubeURL"
			url, err := config.Get[string](ctx, urlKey)
			if err != nil {
				// respond saying internal error
			}
			// respond with the url if it exists
			if url != "" {
				utils.RespondLink(event, "Join Synctube", url, false)
				return
			} else {
				utils.RespondTemp(event, "This guild is not configured or the synctube url is not set", true, 5*time.Second)
			}
		*/
	},
}

func init() {
	List["sync"] = syncCommand
}
