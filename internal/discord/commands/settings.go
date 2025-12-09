package commands

import (
	"fmt"
	"sprout/internal/app"
	"sprout/internal/platform/auth"

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
		token, err := a.AuthManager.NewParamSession(a.DB, event.User().ID)
		if err != nil {
			a.Log.Error("Failed to create settings session: ", err)
			event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("Failed to create settings session. Please try again later.").
				SetEphemeral(true).
				Build())
			return err
		}
		// send link
		url := fmt.Sprintf("%s/login?%s=%s", a.BaseURL, auth.ParamName, token)
		event.CreateMessage(discord.NewMessageCreateBuilder().
			AddActionRow(discord.NewLinkButton("Manage Settings", url)).
			SetEphemeral(true).
			Build())
		return nil
	},
})
