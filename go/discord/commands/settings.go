package commands

import (
	"sprout/go/app"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var Settings = register(BotCommand{
	IsGlobal:     true,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "settings",
		Description: "View or change bot settings",
	},
	Handler: func(a *app.App, event *events.ApplicationCommandInteractionCreate) error {
		// create settings session
		token, err := a.Net.SettingsAuth.NewSession(event.User().ID)
		if err != nil {
			a.Log.Error("Failed to create settings session: ", err)
			event.Respond(discord.InteractionResponseTypeCreateMessage, discord.NewMessageCreateBuilder().
				SetContent("Failed to create settings session. Please try again later.").
				SetEphemeral(true).
				Build())
			return err
		}
		// send link
		url := a.Net.BaseURL + "/settings?a=" + token
		event.Respond(discord.InteractionResponseTypeCreateMessage, discord.NewMessageCreateBuilder().
			AddActionRow(discord.NewLinkButton("Manage Settings", url)).
			SetEphemeral(true).
			Build())
		return nil
	},
})
