package commands

import (
	"context"
	"encoding/json"
	"halsey/go/disc/respond"
	"halsey/go/storage/database"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var syncCommand = BotCommand{
	IsGlobal:     false,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "sync",
		Description: "Links the guild's synctube.",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) {
		// get db
		db := database.FromContext(ctx)
		if db == nil {
			xlog.Error(ctx, "database not found in context")
			respond.Normal(ctx, event, "An internal error occurred while trying to get the database.", true)
			return
		}

		urlKey := []byte(event.GuildID().String() + ".synctubeURL")
		xlog.Debugf(ctx, "Getting synctube URL for guild. Key: %s", urlKey)

		buf, err := db.Read(database.GuildsDBIName, urlKey)
		if err != nil {
			if lmdb.IsNotFound(err) {
				xlog.Error(ctx, "Synctube URL not found in database for guild", event.GuildID())
				respond.Normal(ctx, event, "This guild has not been configured yet. Try restarting the bot.", true)
				return
			}
			xlog.Errorf(ctx, "Error reading synctube URL from database: %s", err)
			respond.Normal(ctx, event, "An internal error occurred while trying to get the synctube URL.", true)
			return
		}
		// unmarshal the url
		var url string
		if err := json.Unmarshal(buf, &url); err != nil {
			xlog.Errorf(ctx, "Error unmarshaling synctube URL from database: %s", err)
			respond.Normal(ctx, event, "An internal error occurred while trying to get the synctube URL.", true)
			return
		}
		xlog.Debugf(ctx, "Synctube URL for guild %s: %s", event.GuildID(), url)
		if url == "" {
			respond.Normal(ctx, event, "This synctube url is not set for this guild", true)
			return
		}
		respond.Link(ctx, event, "Join Synctube", url, false)
	},
}

func init() {
	List["sync"] = syncCommand
}
