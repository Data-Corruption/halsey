package listeners

import (
	"sprout/go/app"
	"sprout/go/discord/commands"
	"sprout/go/platform/http/server/auth"

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
			if isAdmin, err := auth.IsUserAdminByID(a.Config, event.User().ID); err != nil {
				a.Log.Errorf("Error checking if user is admin: %s", err)
				if err := event.Respond(discord.InteractionResponseTypeCreateMessage, discord.NewMessageCreateBuilder().
					SetContent("Internal server error.").
					SetEphemeral(true).
					Build()); err != nil {
					a.Log.Errorf("Error responding to interaction: %s", err)
				}
				return
			} else if !isAdmin {
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
