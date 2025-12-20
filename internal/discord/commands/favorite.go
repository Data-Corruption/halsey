package commands

import (
	"fmt"
	"sprout/internal/app"
	"sprout/internal/discord/attachments"
	"sprout/internal/discord/emojis"
	"sprout/internal/discord/response"
	"sprout/internal/platform/database"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

var Favorite = register(BotCommand{
	IsGlobal:     false,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.MessageCommandCreate{
		Name: "favorite",
	},
	Handler: func(a *app.App, event *events.ApplicationCommandInteractionCreate) error {
		if err := event.DeferCreateMessage(true); err != nil {
			return err
		}

		data := event.MessageCommandInteractionData()
		message := data.TargetMessage()

		// get fav channel
		guild, err := database.ViewGuild(a.DB, *event.GuildID())
		if err != nil {
			a.Log.Error("Failed to get guild: ", err)
			return createFollowupMessage(a, event.Token(), "Internal error", true)
		}
		if (guild.FavChannelID == 0) || (guild.FavChannelID == *event.GuildID()) {
			return createFollowupMessage(a, event.Token(), "Favorite channel not set", true)
		}
		favChannel, ok := a.Client.Caches.Channel(guild.FavChannelID)
		if !ok {
			a.Log.Error("Favorite channel not found")
			return createFollowupMessage(a, event.Token(), "Favorite channel not found", true)
		}

		// get fav dbi
		fDBI, ok := a.DB.GetDBis()[database.FavoritesDBIName]
		if !ok {
			a.Log.Error("Favorite DB not found")
			return createFollowupMessage(a, event.Token(), "Error: Favorite DB not found", true)
		}

		key := []byte(message.ID.String())

		// check for existing favorite
		var existingFavID snowflake.ID
		if err := a.DB.View(func(txn *lmdb.Txn) error {
			return database.TxnGetAndUnmarshal(txn, fDBI, key, &existingFavID)
		}); err != nil && !lmdb.IsNotFound(err) {
			a.Log.Error("Failed to check existing favorite: ", err)
			return createFollowupMessage(a, event.Token(), "Internal error", true)
		}

		// handle existing favorite
		if existingFavID != 0 {
			// has this been deleted manually by a user?
			if _, err := a.Client.Rest.GetMessage(favChannel.ID(), existingFavID); err == nil {
				// still exists, return early
				msg := fmt.Sprintf("This message is already favorited! https://discord.com/channels/%s/%s/%s", *event.GuildID(), favChannel.ID(), existingFavID)
				return createFollowupMessage(a, event.Token(), msg, true)
			}
			// message was deleted, we'll create a new one below
		}

		// create favorite in fav channel
		favMsgOut, err := a.Client.Rest.CreateMessage(favChannel.ID(), buildFavoriteMessage(a, message))
		if err != nil {
			a.Log.Error("Failed to create favorite message: ", err)
			return createFollowupMessage(a, event.Token(), "Failed to create favorite", true)
		}

		// write to db
		if err := a.DB.Update(func(txn *lmdb.Txn) error {
			return database.TxnMarshalAndPut(txn, fDBI, key, favMsgOut.ID)
		}); err != nil {
			a.Log.Error("Failed favorite DB txn: ", err)
			return createFollowupMessage(a, event.Token(), "Internal error", true)
		}

		// react to original message
		favEmoji, ok := emojis.GetRandFavEmoji(a)
		if !ok {
			a.Log.Error("Failed to get random favorite emoji")
		} else {
			if err := response.ReactToMessage(a, message.ChannelID, message.ID, favEmoji.String()); err != nil {
				a.Log.Error("Failed to react to original message: ", err)
			}
		}

		return createFollowupMessage(a, event.Token(), "Favorite added!", true)
	},
})

func buildFavoriteMessage(a *app.App, message discord.Message) discord.MessageCreate {
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

	buttonFound := false
	var oEmoji *discord.ComponentEmoji
	var oUrl string
	for _, c := range message.Components {
		discordActionRow, ok := c.(discord.ActionRowComponent)
		if !ok {
			continue
		}
		for _, comp := range discordActionRow.Components {
			if linkButton, ok := comp.(discord.ButtonComponent); ok {
				if (linkButton.Style == discord.ButtonStyleLink) && ((linkButton.Emoji != nil) || (linkButton.Label == "▲")) {
					oEmoji = linkButton.Emoji
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

	msgBuilder := discord.NewMessageCreateBuilder()
	msgBuilder.SetFlags(discord.MessageFlagIsComponentsV2)

	// top row buttons
	jumpToMsgBtn := discord.NewLinkButton("◀",
		fmt.Sprintf("https://discord.com/channels/%s/%s/%s", *message.GuildID, message.ChannelID, message.ID),
	)
	removeBtn := discord.NewSecondaryButton("✖", fmt.Sprintf("remove_favorite.%s.%s", message.ID.String(), message.ChannelID.String()))
	if buttonFound {
		if oEmoji != nil {
			msgBuilder.AddComponents(discord.NewActionRow(
				jumpToMsgBtn,
				discord.NewLinkButton("", oUrl).WithEmoji(*oEmoji),
				removeBtn,
			))
		} else {
			msgBuilder.AddComponents(discord.NewActionRow(
				jumpToMsgBtn,
				discord.NewLinkButton("▲", oUrl),
				removeBtn,
			))
		}
	} else {
		msgBuilder.AddComponents(discord.NewActionRow(
			jumpToMsgBtn,
			removeBtn,
		))
	}

	// copy components
	if len(message.Components) > 0 {
		if !buttonFound {
			msgBuilder.AddComponents(message.Components...)
		} else {
			// first component is the "Open in..." button, if there are more, copy them
			if len(message.Components) > 1 {
				msgBuilder.AddComponents(message.Components[1:]...)
			}
		}
	}

	// copy media
	if len(media) > 0 {
		var mediaItems []discord.MediaGalleryItem
		for _, attachment := range media {
			mediaItems = append(mediaItems, discord.MediaGalleryItem{
				Media: discord.UnfurledMediaItem{URL: attachment.URL},
			})
		}
		msgBuilder.AddComponents(discord.NewMediaGallery(mediaItems...))
	}

	// copy content
	if content != "" {
		msgBuilder.AddComponents(discord.NewTextDisplay(content))
	}

	// add author footer (name and timestamp) skip if message if from self
	if message.Author.ID != a.Client.ApplicationID {
		msgBuilder.AddComponents(discord.NewTextDisplay(
			fmt.Sprintf("`%s` • <t:%d:f>", message.Author.Username, message.CreatedAt.Unix()),
		))
	}

	return msgBuilder.Build()
}
