package commands

import (
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var Sync = register(BotCommand{
	IsGlobal:     false,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "sync",
		Description: "Get the synctube url for this server",
	},
	Handler: func(a *app.App, event *events.ApplicationCommandInteractionCreate) error {
		// get server
		server, err := database.ViewGuild(a.DB, *event.GuildID())
		if err != nil {
			return err
		}
		// get synctube url
		synctubeURL := server.SynctubeURL
		if synctubeURL == "" {
			event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("No synctube url found for this server").Build())
			return nil
		}
		// send message
		event.CreateMessage(discord.NewMessageCreateBuilder().
			AddActionRow(discord.NewLinkButton("Open Synctube", synctubeURL)).Build())
		return nil
	},
})
