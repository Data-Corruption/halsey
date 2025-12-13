package components

import (
	"fmt"
	"sprout/internal/app"
	"sprout/internal/discord/response"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var RemoveFavorite = register(BotComponent{
	ID: "remove_favorite",
	Handler: func(a *app.App, event *events.ComponentInteractionCreate, idParts []string) error {
		// parse interaction ID parts
		if len(idParts) < 2 {
			event.CreateMessage(buildMsg("An error occurred."))
			return fmt.Errorf("remove_favorite interaction without content copy message ID: %s", event.Data.CustomID())
		}
		sourceMessageID := idParts[0]
		sourceChannelID := idParts[1]

		// delete favorite message
		if err := a.Client.Rest.DeleteMessage(event.Message.ChannelID, event.Message.ID); err != nil {
			event.CreateMessage(buildMsg("An error occurred while trying to remove the favorite."))
			return fmt.Errorf("error deleting author message %s: %w", event.Message.ID, err)
		}

		// remove sourceMessageID from database
		if err := a.DB.Delete(database.FavoritesDBIName, []byte(sourceMessageID)); err != nil {
			event.CreateMessage(buildMsg("An error occurred while trying to remove the favorite."))
			return fmt.Errorf("error deleting favorite message ID %s from database: %w", sourceMessageID, err)
		}
		a.Log.Infof("Removed favorite message %s from channel %s", sourceMessageID, event.Message.ChannelID)

		// send confirmation to bot channel
		msg := fmt.Sprintf("Unfavorited https://discord.com/channels/%s/%s/%s", event.GuildID(), sourceChannelID, sourceMessageID)
		if _, err := response.MessageBotChannel(a, *event.GuildID(), discord.NewMessageCreateBuilder().SetContent(msg).Build()); err != nil {
			a.Log.Errorf("Error sending unfavorite message to bot channel: %s", err)
		}

		// acknowledge interaction, is required, this says we might respond, use this when you don't want to send a message back
		event.Respond(discord.InteractionResponseTypeDeferredUpdateMessage, nil)

		return nil
	},
})

func buildMsg(msg string) discord.MessageCreate {
	return discord.NewMessageCreateBuilder().
		SetContent(msg).
		SetEphemeral(true).
		Build()
}
