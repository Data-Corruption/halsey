package commands

import (
	"sprout/internal/app"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var Download = register(BotCommand{
	IsGlobal:     false,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.MessageCommandCreate{
		Name: "download",
	},
	Handler: func(a *app.App, event *events.ApplicationCommandInteractionCreate) error {
		if err := event.DeferCreateMessage(true); err != nil {
			return err
		}

		return createFollowupMessage(a, event.Token(), "Work in progress...", true)
	},
})
