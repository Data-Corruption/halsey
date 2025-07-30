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
	"github.com/disgoorg/snowflake/v2"
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

		if len(parts) < 4 {
			xlog.Errorf(ctx, "remove_favorite interaction without content copy message ID: %s", event.Data.CustomID())
			event.CreateMessage(genericErr)
			return
		}

		// content copy message ID
		cMsgIdStr := parts[1]
		cMsgID, err := snowflake.Parse(cMsgIdStr)
		if err != nil {
			xlog.Errorf(ctx, "Error parsing content copy message ID %s: %s", cMsgIdStr, err)
			event.CreateMessage(genericErr)
			return
		}
		// source message ID
		sMsgIdStr := parts[2]
		sChnIdStr := parts[3]
		// author / controls message ID
		aMsgID := event.Message.ID

		// delete both
		if err := disc.Client.Rest.DeleteMessage(event.Message.ChannelID, cMsgID); err != nil {
			xlog.Errorf(ctx, "Error deleting content copy message %s: %s", cMsgID, err)
			event.CreateMessage(genericErr)
			return
		}
		if err := disc.Client.Rest.DeleteMessage(event.Message.ChannelID, aMsgID); err != nil {
			xlog.Errorf(ctx, "Error deleting author message %s: %s", aMsgID, err)
			event.CreateMessage(genericErr)
			return
		}

		// remove sMsgIdStr from database
		db := database.FromContext(ctx)
		if db == nil {
			xlog.Errorf(ctx, "Error getting database: %s", err)
			event.CreateMessage(genericErr)
			return
		}
		if err := db.Delete(database.FavoritesDBIName, []byte(sMsgIdStr)); err != nil {
			xlog.Errorf(ctx, "Error deleting favorite message ID %s from database: %s", sMsgIdStr, err)
			event.CreateMessage(genericErr)
			return
		}
		xlog.Infof(ctx, "Removed favorite message %s from channel %s", sMsgIdStr, event.Message.ChannelID)
		respond.BotChannel(ctx, fmt.Sprintf("Unfavorited https://discord.com/channels/%s/%s/%s", event.GuildID(), sChnIdStr, sMsgIdStr))
		event.CreateMessage(discord.NewMessageCreateBuilder().
			SetContent("Unfavorited!").
			SetEphemeral(true).
			Build(),
		)
	default:
		xlog.Warnf(ctx, "Unknown component interaction: %s", event.Data.CustomID())
	}
}
