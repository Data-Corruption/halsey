package listeners

import (
	"fmt"
	"sprout/go/app"
	"sprout/go/platform/database"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func OnComponentInteraction(a *app.App, event *events.ComponentInteractionCreate) {
	go func() {
		// split event.Data.CustomID() on '.', prefix is what we switch on. Could also not have a second part.
		parts := strings.Split(event.Data.CustomID(), ".")
		if len(parts) < 1 {
			a.Log.Warnf("Component interaction with empty CustomID: %s", event.Data.CustomID())
			return
		}
		switch parts[0] {
		case "remove_favorite":
			removeFavorite(a, event, parts)
		default:
			a.Log.Warnf("Unknown component interaction: %s", event.Data.CustomID())
		}
	}()
}

// note - thank you past me for making this extensible 8==D <3

func removeFavorite(a *app.App, event *events.ComponentInteractionCreate, parts []string) {
	genericErr := discord.NewMessageCreateBuilder().
		SetContent("An error occurred while trying to remove the favorite.").
		SetEphemeral(true).
		Build()

	if len(parts) < 3 {
		a.Log.Errorf("remove_favorite interaction without content copy message ID: %s", event.Data.CustomID())
		event.CreateMessage(genericErr)
		return
	}
	sourceMessageID := parts[1]
	sourceChannelID := parts[2]

	// delete favorite message
	if err := a.Client.Rest.DeleteMessage(event.Message.ChannelID, event.Message.ID); err != nil {
		a.Log.Errorf("Error deleting author message %s: %s", event.Message.ID, err)
		event.CreateMessage(genericErr)
		return
	}

	// remove sourceMessageID from database
	if err := a.DB.Delete(database.FavoritesDBIName, []byte(sourceMessageID)); err != nil {
		a.Log.Errorf("Error deleting favorite message ID %s from database: %s", sourceMessageID, err)
		event.CreateMessage(genericErr)
		return
	}
	a.Log.Infof("Removed favorite message %s from channel %s", sourceMessageID, event.Message.ChannelID)
	respond.BotChannel(ctx, fmt.Sprintf("Unfavorited https://discord.com/channels/%s/%s/%s", event.GuildID(), sourceChannelID, sourceMessageID))
	event.Respond(discord.InteractionResponseTypeDeferredUpdateMessage, nil)
}
