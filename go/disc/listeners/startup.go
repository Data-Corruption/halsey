package listeners

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/disc/commands"
	"halsey/go/storage/config"
	"halsey/go/storage/database"
	"strings"
	"time"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func OnReady(ctx context.Context, event *events.Ready) {
	fmt.Println("Halsey is now running. Press Ctrl+C to exit.")
	xlog.Info(ctx, "Halsey is now running.")
}

func OnGuildsReady(ctx context.Context, registerCommands bool, event *events.GuildsReady) {
	// get db and guild DBI
	db, gDBI, err := database.GetDbAndDBI(ctx, database.GuildsDBIName)
	if err != nil {
		xlog.Error(ctx, "failed to get database and DBI: %w", err)
		return
	}

	// start txn, add guilds to database if they don't exist, update guild names if they have changed
	if err := db.Update(func(txn *lmdb.Txn) error {
		for guild := range disc.Client.Caches.GuildCache().All() {
			// get name, will also inform us if it's in the db or not
			id := guild.ID.String()
			if id == "" {
				xlog.Errorf(ctx, "guild ID is empty, skipping: %v", guild)
				continue
			}
			nameKey := []byte(id + ".name")
			var name string
			err := database.GetAndUnmarshal(txn, gDBI, nameKey, &name)
			if err != nil {
				if lmdb.IsNotFound(err) {
					// write guild to database with default values
					if err := addNewGuildToDB(ctx, txn, gDBI, id, guild.Name); err != nil {
						return fmt.Errorf("failed to add new guild to database: %w", err)
					}
				} else {
					return fmt.Errorf("failed to get guild name for %s: %w", guild.ID, err)
				}
			} else {
				// update name if it has changed
				if name != guild.Name {
					xlog.Info(ctx, fmt.Sprintf("Guild %s name changed to %s, updating...", name, guild.Name))
					if err := database.MarshalAndPut(txn, gDBI, nameKey, guild.Name); err != nil {
						return fmt.Errorf("failed to marshal guild name for %s: %w", guild.ID, err)
					}
				}
			}
		}
		return nil
	}); err != nil {
		xlog.Error(ctx, "failed to update database in onGuildsReady: %w", err)
		return
	}

	// register commands if needed
	if registerCommands {
		xlog.Info(ctx, "Registering commands...")
		registerCmds(ctx)
	}

	// handle update command response
	// get updateFollowup from config
	updateFollowup, err := config.Get[string](ctx, "updateFollowup")
	if err != nil {
		xlog.Error(ctx, "failed to get updateFollowup from config: %w", err)
		return
	}
	if updateFollowup != "" {
		// split event token and message ID
		parts := strings.Split(updateFollowup, "|")
		if len(parts) != 2 {
			xlog.Error(ctx, "updateFollowup format is invalid, expected '<event token>|<message ID>', got: ", updateFollowup)
			return
		}

		// clear updateFollowup from config
		if err := config.Set(ctx, "updateFollowup", ""); err != nil {
			xlog.Error(ctx, "failed to clear updateFollowup in config: %w", err)
			return
		}

		// parse into snowflake
		messageID, err := snowflake.Parse(parts[1])
		if err != nil {
			xlog.Error(ctx, "failed to parse message ID into snowflake: %w", err)
			return
		}

		// get current version from context
		version, ok := ctx.Value("appVersion").(string)
		if !ok {
			xlog.Error(ctx, "failed to get appVersion from context")
			return
		}

		// update the followup message
		_, err = disc.Client.Rest.UpdateFollowupMessage(disc.Client.ApplicationID, parts[0], messageID, discord.NewMessageUpdateBuilder().
			SetContentf("Updated to version %s successfully!", version).Build())
		if err != nil {
			xlog.Error(ctx, "failed to update followup message: %w", err)
		}
	}

	// TODO: start backup service

}

func registerCmds(ctx context.Context) {
	// get command creation data
	var globalCommands = []discord.ApplicationCommandCreate{}
	var guildCommands = []discord.ApplicationCommandCreate{}
	for _, command := range commands.List {
		if command.IsGlobal {
			globalCommands = append(globalCommands, command.Data)
		} else {
			guildCommands = append(guildCommands, command.Data)
		}
	}
	// register global commands
	xlog.Debugf(ctx, "global commands being registered: %v", globalCommands)
	if _, err := disc.Client.Rest.SetGlobalCommands(disc.Client.ApplicationID, globalCommands); err != nil {
		xlog.Errorf(ctx, "error registering global commands: %s", err)
	}
	// register guild commands
	xlog.Debugf(ctx, "guild commands being registered: %v", guildCommands)
	for guild := range disc.Client.Caches.GuildCache().All() {
		if _, err := disc.Client.Rest.SetGuildCommands(disc.Client.ApplicationID, guild.ID, guildCommands); err != nil {
			xlog.Errorf(ctx, "error registering guild commands for guild %s: %s", guild.Name, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func addNewGuildToDB(ctx context.Context, txn *lmdb.Txn, dbi lmdb.DBI, id, name string) error {
	// write guild to database with default values
	xlog.Info(ctx, fmt.Sprintf("Guild %s not found in database, adding...", id))
	if err := database.MarshalAndPut(txn, dbi, []byte(id+".name"), name); err != nil {
		return fmt.Errorf("failed to marshal guild name for %s: %w", id, err)
	}
	if err := database.MarshalAndPut(txn, dbi, []byte(id+".backup.run"), ""); err != nil {
		return fmt.Errorf("failed to marshal guild backup run for %s: %w", id, err)
	}
	if err := database.MarshalAndPut(txn, dbi, []byte(id+".backup.enabled"), true); err != nil {
		return fmt.Errorf("failed to marshal guild backup enabled for %s: %w", id, err)
	}
	if err := database.MarshalAndPut(txn, dbi, []byte(id+".favoriteChannelID"), ""); err != nil {
		return fmt.Errorf("failed to marshal guild favorite channel ID for %s: %w", id, err)
	}
	if err := database.MarshalAndPut(txn, dbi, []byte(id+".uploadLimitMB"), 8); err != nil {
		return fmt.Errorf("failed to marshal guild upload limit for %s: %w", id, err)
	}
	if err := database.MarshalAndPut(txn, dbi, []byte(id+".synctubeURL"), "https://youtu.be/PE0HpSknfw0?si=OBXhL6A7NTu44lPH"); err != nil {
		return fmt.Errorf("failed to marshal guild synctube URL for %s: %w", id, err)
	}
	return nil
}
