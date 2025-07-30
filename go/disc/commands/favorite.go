package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"halsey/go/disc"
	"halsey/go/disc/respond"
	"halsey/go/storage/database"
	"time"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

var favoriteCommand = BotCommand{
	IsGlobal:     false,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.MessageCommandCreate{
		Name: "favorite",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) {

		data := event.MessageCommandInteractionData()
		message := data.TargetMessage()

		// get favorite channel
		favChannel, err := getFavChannel(ctx, event.GuildID().String())
		if err != nil {
			xlog.Error(ctx, fmt.Sprintf("Error getting favorite channel: %s", err))
			respond.Normal(ctx, event, "An error occurred while trying to get the favorite channel.", true)
			return
		}

		// get db for txn
		db, fDBI, err := database.GetDbAndDBI(ctx, database.FavoritesDBIName)
		if err != nil {
			xlog.Error(ctx, fmt.Sprintf("Error getting database: %s", err))
			respond.Normal(ctx, event, "An internal error occurred while trying to get the database.", true)
			return
		}

		alreadyInFavID := ""
		if err = db.Update(func(txn *lmdb.Txn) error {
			// check if message is already in favorite channel. If so, return early
			key := []byte(message.ID.String())
			buf, err := txn.Get(fDBI, key)
			if err == nil {
				// ok we have a msg id, lets double check in case it was deleted
				var idRaw string
				if err := json.Unmarshal(buf, &idRaw); err != nil {
					return fmt.Errorf("error unmarshaling favorite message ID: %w", err)
				}
				id, err := snowflake.Parse(idRaw)
				if err != nil {
					return fmt.Errorf("error parsing favorite message ID: %s", idRaw)
				}
				favMessage, err := disc.Client.Rest.GetMessage(favChannel.ID(), id)
				if err != nil {
					// if message is unknown, delete it from the database
					if disc.IsUnknownMessage(err) {
						xlog.Warnf(ctx, "Message %s is unknown in channel %s, deleting from favorites", idRaw, favChannel.ID())
						if err := txn.Del(fDBI, key, nil); err != nil {
							return fmt.Errorf("error deleting favorite message ID from database: %w", err)
						}
					} else {
						return fmt.Errorf("error getting message from favorite channel: %w", err)
					}
				} else {
					alreadyInFavID = favMessage.ID.String()
					xlog.Debugf(ctx, "Message %s is already favorited in channel %s", alreadyInFavID, favChannel.ID())
					return nil // already favorited, no need to continue
				}
			} else {
				if !lmdb.IsNotFound(err) {
					return fmt.Errorf("error checking if message is already in favorite channel: %w", err)
				}
			}

			// send message to favorite channel
			cMsg, err := disc.Client.Rest.CreateMessage(favChannel.ID(), discord.NewMessageCreateBuilder().
				SetContent(buildContentString(message)).
				Build(),
			)
			if err != nil {
				fmt.Printf("Error creating message in favorite channel: %s\n", err)
			}
			time.Sleep(50 * time.Millisecond)
			fMsg, err := disc.Client.Rest.CreateMessage(favChannel.ID(), discord.NewMessageCreateBuilder().
				AddActionRow(
					discord.NewLinkButton("Jump to message",
						fmt.Sprintf("https://discord.com/channels/%s/%s/%s", event.GuildID(), message.ChannelID, message.ID),
					),
					discord.NewSecondaryButton("✖", "remove_favorite."+cMsg.ID.String()+"."+message.ID.String()+"."+message.ChannelID.String()),
				).
				Build(),
			)
			if err != nil {
				fmt.Printf("Error creating author embed in favorite channel: %s\n", err)
			}

			// store in db
			data, err := json.Marshal(fMsg.ID.String())
			if err != nil {
				fmt.Printf("Error marshaling favorite message ID: %s\n", err)
			}
			if err := txn.Put(fDBI, key, data, 0); err != nil {
				return fmt.Errorf("error writing favorite message ID to database: %w", err)
			}
			xlog.Debugf(ctx, "Added message %s to favorites in channel %s", fMsg.ID, favChannel.ID())
			return nil
		}); err != nil {
			xlog.Error(ctx, fmt.Sprintf("Error updating database: %s", err))
			respond.Normal(ctx, event, "An internal error occurred while trying to update the database.", true)
			return
		}

		if alreadyInFavID != "" {
			respond.Normal(ctx, event, fmt.Sprintf("This message is already favorited! https://discord.com/channels/%s/%s/%s", event.GuildID(), favChannel.ID(), alreadyInFavID), true)
			return
		} else {
			respond.Temp(ctx, event, "Favorited!", true, 3*time.Second)
		}

		emoji, err := disc.GetRandAppEmoji(ctx)
		if err != nil {
			xlog.Warnf(ctx, "Error getting random app emoji: %s", err)
			return
		}
		respond.React(disc.Client, message.ChannelID, message.ID, emoji)
		xlog.Debugf(ctx, "Added favorite emoji %s to message %s in channel %s", emoji, message.ID, message.ChannelID)
	},
}

func init() {
	List["favorite"] = favoriteCommand
}

func getFavChannel(ctx context.Context, guildID string) (discord.Channel, error) {
	db := database.FromContext(ctx)
	if db == nil {
		return nil, fmt.Errorf("database not found in context")
	}
	key := []byte(guildID + ".favoriteChannelID")
	favChannelIDRaw, err := db.Read(database.GuildsDBIName, key)
	if err != nil {
		return nil, fmt.Errorf("error getting favorite channel ID from database: %w", err)
	}
	var favChannelID string
	if err := json.Unmarshal(favChannelIDRaw, &favChannelID); err != nil {
		return nil, fmt.Errorf("error unmarshaling favorite channel ID: %w", err)
	}
	if favChannelID == "" {
		return nil, fmt.Errorf("this server does not have a favorite channel set")
	}
	favChannelIDSnowflake, err := snowflake.Parse(favChannelID)
	if err != nil {
		return nil, fmt.Errorf("error parsing favorite channel ID: %w", err)
	}
	// get the channel from the cache
	favChannel, ok := disc.Client.Caches.Channel(favChannelIDSnowflake)
	if !ok {
		return nil, fmt.Errorf("favorite channel not found in cache: %s", favChannelID)
	}
	return favChannel, nil
}

func buildContentString(message discord.Message) string {
	content := message.Content + " "
	for _, attachment := range message.Attachments {
		content += attachment.URL + " "
	}
	return content
}
