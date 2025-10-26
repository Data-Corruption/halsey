package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"halsey/go/disc"
	"halsey/go/disc/attachments"
	"halsey/go/disc/respond"
	"halsey/go/storage/database"
	"math/rand"
	"strings"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

var cuteMessages = []string{
	"Oh yeah, this is a good one!",
	"Rawr xD uwu owo *pounces on your message*",
	":black_heart:",
	"Certified banger",
	"Umm... based department?",
	"ngl this is too gay",
	"ngl this is too straight",
	"slay queen",
	"God is dead. Murdered by our own hubris. Our insatiable greed for understanding, incinerating all fiction in its gaze. We are the architects of our own colorless prison, and this message is a testament to that.",
	"hehe, i like this one",
	"this is why blue lives matter",
	"this is why blue lives don't matter",
	"Did you know most of my viewers arn't subscribed? If you like this message, consider subscribing! and ring-a-ding that bell!",
	"and i oop",
	"2003 > brotherhood",
	"this is slop",
	"is this satire?",
	"oof this is cringe",
	"whomp whomp",
	"did you just say whomp whomp?",
	"my pants, pissed.",
	"it is what it is",
	"I'm in this and I don't like it",
	"Ezekiel 23:20 - There she lusted after her lovers, whose genitals were like those of donkeys and whose emission was like that of horses.",
	"wake up",
	"erm, ya gay",
	"there's nothing funny about 9/11",
	"step on me",
	"i'm not a furry, i just like the art",
	"i uh, i don't get it",
	"that's actually really funny",
	"pop off sis",
	"*burps*",
	"facts",
	"truee",
	"*explodes*",
	"when you see it, you'll shit bricks",
	"eat the rich",
	"*wink*",
	"poggers",
	"this got me feeling a type of way",
	"As an AI language model, I am unable to react to this.",
	"hold on, my doordash just got here",
	"my life be like ooh ahh",
	"it's true what they say. girls get it done",
	"somebody call the waaambulance",
	"you never know what's voice activated these days",
	"welp... i'm hard",
	"this is making me hungry",
	"feed em to the goats",
	"this is actually really offensive",
	"that boy stuck in time",
	"Down on the bayou...",
	"gotta put a little more ass in that",
	"i just shit myself",
	"not unlike the academy award winning film Suicide Squad",
	"love me some orange juice!",
	"Don't make a girl a promise, if you know you can't keep it",
	"It's fine.. I just didn't think It'd be Chinese is all",
	"*has violent explosive diarrhea*",
	"If I fits, I sits.",
	"If i had a body, I'd use it to stay away from this",
	"Including or not including gang violence?",
	"this feels like a gio post",
	"thank god i'm not fucking canadian, holy shit i would immediately launch myself off the nearest bridge",
	"beep beep",
}

var favoriteCommand = BotCommand{
	IsGlobal:     false,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.MessageCommandCreate{
		Name: "favorite",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
		if err := resDeferMessage(ctx, event); err != nil {
			return fmt.Errorf("error deferring interaction: %s", err)
		}

		data := event.MessageCommandInteractionData()
		message := data.TargetMessage()
		genericErr := discord.NewMessageCreateBuilder().
			SetContent("An error occurred while trying to favorite this message. See logs for more details.").
			Build()

		// get favorite channel
		favChannel, err := getFavChannel(ctx, event.GuildID().String())
		if err != nil {
			xlog.Error(ctx, fmt.Sprintf("Error getting favorite channel: %s", err))
			_, err = resFollowupMessage(ctx, event, genericErr)
			return err
		}

		// if the message is from the favorite channel, return early
		if message.ChannelID == favChannel.ID() {
			_, err = resFollowupMessage(ctx, event, discord.NewMessageCreateBuilder().
				SetContent("This message is already in the favorites channel.").
				Build())
			return err
		}

		// get db for txn
		db, fDBI, err := database.GetDbAndDBI(ctx, database.FavoritesDBIName)
		if err != nil {
			xlog.Error(ctx, fmt.Sprintf("Error getting database: %s", err))
			_, err = resFollowupMessage(ctx, event, genericErr)
			return err
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

			// parse source message
			text, media := parseSourceMessage(message)

			buttonFound := false
			oLabel := ""
			oUrl := ""
			for _, c := range message.Components {
				discordActionRow, ok := c.(discord.ActionRowComponent)
				if !ok {
					continue
				}
				for _, comp := range discordActionRow.Components {
					if linkButton, ok := comp.(discord.ButtonComponent); ok {
						if linkButton.Style == discord.ButtonStyleLink && strings.HasPrefix(linkButton.Label, "Open In") {
							oLabel = linkButton.Label
							oUrl = linkButton.URL
							buttonFound = true
							break
						}
					}
				}
				if buttonFound {
					break
				}
			}

			// build message
			msgBuilder := discord.NewMessageCreateBuilder()
			msgBuilder.SetFlags(discord.MessageFlagIsComponentsV2)
			if buttonFound {
				msgBuilder.AddComponents(discord.NewActionRow(
					discord.NewLinkButton(oLabel, oUrl),
					discord.NewLinkButton("Jump to message",
						fmt.Sprintf("https://discord.com/channels/%s/%s/%s", event.GuildID(), message.ChannelID, message.ID),
					),
					discord.NewSecondaryButton("✖", fmt.Sprintf("remove_favorite.%s.%s", message.ID.String(), message.ChannelID.String())),
				))
			} else {
				msgBuilder.AddComponents(discord.NewActionRow(
					discord.NewLinkButton("Jump to message",
						fmt.Sprintf("https://discord.com/channels/%s/%s/%s", event.GuildID(), message.ChannelID, message.ID),
					),
					discord.NewSecondaryButton("✖", fmt.Sprintf("remove_favorite.%s.%s", message.ID.String(), message.ChannelID.String())),
				))
			}
			if len(message.Components) > 0 {
				if !buttonFound {
					msgBuilder.AddComponents(message.Components...)
				} else {
					if len(message.Components) > 1 {
						msgBuilder.AddComponents(message.Components[1:]...)
					}
				}
			}
			if len(media) > 0 {
				var mediaItems []discord.MediaGalleryItem
				for _, attachment := range media {
					mediaItems = append(mediaItems, discord.MediaGalleryItem{
						Media:       discord.UnfurledMediaItem{URL: attachment.URL},
						Description: "Halsey's biography picture",
					})
				}
				msgBuilder.AddComponents(discord.NewMediaGallery(mediaItems...))
			}
			if text != "" {
				msgBuilder.AddComponents(discord.NewTextDisplay(text))
			}
			// add author bit (name and timestamp) skip if message if from self
			if message.Author.ID != disc.Client.ApplicationID {
				msgBuilder.AddComponents(discord.NewTextDisplay(
					fmt.Sprintf("`%s` • <t:%d:f>", message.Author.Username, message.CreatedAt.Unix()),
				))
			}

			// send message to favorite channel
			fMsg, err := disc.Client.Rest.CreateMessage(favChannel.ID(), msgBuilder.Build())
			if err != nil {
				fmt.Printf("Error creating message in favorite channel: %s\n", err)
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
			_, err = resFollowupMessage(ctx, event, genericErr)
			return err
		}

		if alreadyInFavID != "" {
			_, err = resFollowupMessage(ctx, event, discord.NewMessageCreateBuilder().
				SetContentf("This message is already favorited! https://discord.com/channels/%s/%s/%s", event.GuildID(), favChannel.ID(), alreadyInFavID).
				SetEphemeral(true).
				Build())
			return err
		} else {
			_, err = resFollowupMessage(ctx, event, discord.NewMessageCreateBuilder().
				SetContent(cuteMessages[rand.Intn(len(cuteMessages))]).
				SetEphemeral(false).
				Build())
			if err != nil {
				xlog.Errorf(ctx, "Error sending followup message: %s", err)
			}
		}

		emoji, err := disc.GetRandFavEmoji(ctx)
		if err != nil {
			xlog.Warnf(ctx, "Error getting random favorite emoji: %s", err)
			return nil
		}
		respond.React(disc.Client, message.ChannelID, message.ID, emoji)
		xlog.Debugf(ctx, "Added favorite emoji %s to message %s in channel %s", emoji, message.ID, message.ChannelID)
		return nil
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

func parseSourceMessage(message discord.Message) (string, []discord.Attachment) {
	var content string
	if message.Content != "" {
		content = message.Content + " "
	}
	var media []discord.Attachment
	for _, attachment := range message.Attachments {
		if attachments.IsMedia(attachment) {
			media = append(media, attachment)
		} else {
			content += attachment.URL + " "
		}
	}
	return content, media
}
