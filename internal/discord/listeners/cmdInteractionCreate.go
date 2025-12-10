package listeners

import (
	"sprout/internal/app"
	"sprout/internal/discord/commands"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func OnCommandInteraction(a *app.App, event *events.ApplicationCommandInteractionCreate) {
	a.DiscordWG.Add(1) // track for graceful shutdown

	// acquire semaphore
	select {
	case a.DiscordEventLimiter <- struct{}{}:
	default:
		a.DiscordWG.Done()
		a.Log.Warn("Event limiter reached, dropping command interaction")
		event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("I'm too busy right now! Please try again in a moment.").
			SetEphemeral(true).
			Build())
		return
	}

	go func() {
		defer a.DiscordWG.Done()
		defer func() { <-a.DiscordEventLimiter }()

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
			if err := event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("Bots cannot use this command.").
				SetEphemeral(true).
				Build()); err != nil {
				a.Log.Errorf("Error responding to interaction: %s", err)
			}
			return
		}

		// ensure user exists in db, update username if changed
		var isAdmin bool
		if _, err := database.UpsertUser(a.DB, event.User().ID, func(user *database.User) error {
			user.Username = event.User().Username
			isAdmin = user.IsAdmin
			return nil
		}); err != nil {
			a.Log.Errorf("Error upserting user: %s", err)
			if err := event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("Internal server error.").
				SetEphemeral(true).
				Build()); err != nil {
				a.Log.Errorf("Error responding to interaction: %s", err)
			}
			return
		}

		// admin check
		if command.RequireAdmin && !isAdmin {
			a.Log.Warnf("User %s is not an admin", event.User().Username)
			if err := event.CreateMessage(discord.NewMessageCreateBuilder().
				SetContent("You do not have permission to use this command.").
				SetEphemeral(true).
				Build()); err != nil {
				a.Log.Errorf("Error responding to interaction: %s", err)
			}
			return
		}

		if err := command.Handler(a, event); err != nil {
			a.Log.Errorf("Error handling command %s: %s", cmdName, err)
		}
	}()
}
