package listeners

import (
	"slices"
	"sprout/go/app"
	"sprout/go/discord/commands"
	"sprout/go/platform/database/config"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func OnCommandInteraction(a *app.App, event *events.ApplicationCommandInteractionCreate) {
	go func() {
		// get command
		var cmdName string = event.Data.CommandName()
		a.Log.Infof("Command interaction received: %s", cmdName)
		command, ok := commands.Get(cmdName)
		if !ok {
			a.Log.Warnf("Unknown command: %s", cmdName)
			return
		}

		// bot check
		if command.FilterBots && event.User().Bot {
			if err := event.Respond(discord.InteractionResponseTypeCreateMessage, discord.NewMessageCreateBuilder().
				SetContent("Bots cannot use this command.").
				SetEphemeral(true).
				Build()); err != nil {
				a.Log.Errorf("Error responding to interaction: %s", err)
			}
			return
		}

		// admin check
		if command.RequireAdmin {
			// get general setting
			settings, err := config.Get[config.GeneralSettings](a.Config, "generalSettings")
			if err != nil {
				a.Log.Errorf("Error getting general settings from config: %s", err)
				return
			}
			userID := event.User().ID.String()
			if !slices.Contains(settings.AdminWhitelist, userID) {
				a.Log.Warnf("User %s is not an admin", userID)
				if err := event.Respond(discord.InteractionResponseTypeCreateMessage, discord.NewMessageCreateBuilder().
					SetContent("You do not have permission to use this command.").
					SetEphemeral(true).
					Build()); err != nil {
					a.Log.Errorf("Error responding to interaction: %s", err)
				}
				return
			}
		}

		if err := command.Handler(a, event); err != nil {
			a.Log.Errorf("Error handling command %s: %s", cmdName, err)
		}
	}()
}
