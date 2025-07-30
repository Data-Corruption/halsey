package listeners

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/disc/respond"
	"halsey/go/storage/database"
	"strings"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func OnComponentInteraction(ctx context.Context, event *events.ComponentInteractionCreate) {
	// split event.Data.CustomID() on '.', prefix is what we switch on. Could also not have a second part.
	parts := strings.Split(event.Data.CustomID(), ".")
	if len(parts) < 1 {
		xlog.Warnf(ctx, "Component interaction with empty CustomID: %s", event.Data.CustomID())
		return
	}
	switch parts[0] {
	case "remove_favorite":
		genericErr := discord.NewMessageCreateBuilder().
			SetContent("An error occurred while trying to remove the favorite.").
			SetEphemeral(true).
			Build()

		if len(parts) < 3 {
			xlog.Errorf(ctx, "remove_favorite interaction without content copy message ID: %s", event.Data.CustomID())
			event.CreateMessage(genericErr)
			return
		}
		sourceMessageID := parts[1]
		sourceChannelID := parts[2]

		// delete favorite message
		if err := disc.Client.Rest.DeleteMessage(event.Message.ChannelID, event.Message.ID); err != nil {
			xlog.Errorf(ctx, "Error deleting author message %s: %s", event.Message.ID, err)
			event.CreateMessage(genericErr)
			return
		}

		// remove sourceMessageID from database
		db := database.FromContext(ctx)
		if db == nil {
			xlog.Error(ctx, "Database not found in context")
			event.CreateMessage(genericErr)
			return
		}
		if err := db.Delete(database.FavoritesDBIName, []byte(sourceMessageID)); err != nil {
			xlog.Errorf(ctx, "Error deleting favorite message ID %s from database: %s", sourceMessageID, err)
			event.CreateMessage(genericErr)
			return
		}
		xlog.Infof(ctx, "Removed favorite message %s from channel %s", sourceMessageID, event.Message.ChannelID)
		respond.BotChannel(ctx, fmt.Sprintf("Unfavorited https://discord.com/channels/%s/%s/%s", event.GuildID(), sourceChannelID, sourceMessageID))
		event.Respond(discord.InteractionResponseTypeDeferredUpdateMessage, nil)
	default:
		xlog.Warnf(ctx, "Unknown component interaction: %s", event.Data.CustomID())
	}
}
