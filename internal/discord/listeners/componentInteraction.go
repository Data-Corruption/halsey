package listeners

import (
	"sprout/internal/app"
	"sprout/internal/discord/components"
	"sprout/internal/platform/database"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func OnComponentInteraction(a *app.App, event *events.ComponentInteractionCreate) {
	a.DiscordWG.Add(1) // track for graceful shutdown

	// acquire semaphore
	select {
	case a.DiscordEventLimiter <- struct{}{}:
	default:
		a.DiscordWG.Done()
		a.Log.Warn("Event limiter reached, dropping component interaction")
		event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("I'm too busy right now! Please try again in a moment.").
			SetEphemeral(true).
			Build())
		return
	}

	go func() {
		defer a.DiscordWG.Done()
		defer func() { <-a.DiscordEventLimiter }()

		// split event.Data.CustomID() on '.', prefix is what we switch on. Could also not have a second part.
		idParts := strings.Split(event.Data.CustomID(), ".")
		if len(idParts) < 1 {
			a.Log.Warnf("Component interaction with empty CustomID: %s", event.Data.CustomID())
			return
		}

		// get component
		component, found := components.Get(idParts[0])
		if !found {
			a.Log.Warnf("Unknown component interaction: %s", event.Data.CustomID())
			return
		}

		// ensure user exists in db, update username if changed
		if _, err := database.UpsertUser(a.DB, event.User().ID, func(user *database.User) error {
			user.Username = event.User().Username
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

		// call handler, passing in the rest of the idParts
		if err := component.Handler(a, event, idParts[1:]); err != nil {
			a.Log.Errorf("Error handling component interaction %s: %s", event.Data.CustomID(), err)
		}
	}()
}
