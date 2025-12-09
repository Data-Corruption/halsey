package commands

import (
	"sprout/internal/app"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var Ping = register(BotCommand{
	IsGlobal:     true,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "ping",
		Description: "Check if the bot is responsive.",
	},
	Handler: func(a *app.App, event *events.ApplicationCommandInteractionCreate) error {
		event.CreateMessage(discord.NewMessageCreateBuilder().SetContent("Pong!").Build())
		return nil
	},
})
