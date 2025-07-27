package listeners

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/storage/database"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func OnReady(ctx context.Context, event *events.Ready) {
	fmt.Println("Halsey is now running. Press Ctrl+C to exit.")
	xlog.Info(ctx, "Halsey is now running.")
}

func OnGuildsReady(ctx context.Context, registerCommands bool, event *events.GuildsReady) {
	// get db and guild DBI
	db := database.FromContext(ctx)
	if db == nil {
		xlog.Error(ctx, "database not found in context")
		return
	}
	gDBI, ok := db.GetDBis()[database.GuildsDBIName]
	if !ok {
		xlog.Error(ctx, "guilds DBI not found in database")
		return
	}

	// start txn, add guilds to database if they don't exist, update guild names if they have changed
	if err := db.Update(func(txn *lmdb.Txn) error {
		var lErr error = nil
		disc.Client.Caches().GuildsForEach(func(guild discord.Guild) {
			if lErr != nil {
				return
			}
			// get name, will also inform us if it's in the db or not
			id := guild.ID.String()
			if id == "" {
				lErr = fmt.Errorf("guild ID is empty, skipping: %v", guild)
				return
			}
			nameKey := []byte(id + ".name")
			var name string
			err := database.GetAndUnmarshal(txn, gDBI, nameKey, &name)
			if err != nil {
				if lmdb.IsNotFound(err) {
					// write guild to database with default values
					xlog.Info(ctx, fmt.Sprintf("Guild %s not found in database, adding...", guild.ID))
					if err := database.MarshalAndPut(txn, gDBI, nameKey, guild.Name); err != nil {
						lErr = fmt.Errorf("failed to marshal guild name for %s: %w", guild.ID, err)
						return
					}
					if err := database.MarshalAndPut(txn, gDBI, []byte(id+".backup.run"), ""); err != nil {
						lErr = fmt.Errorf("failed to marshal guild backup run for %s: %w", guild.ID, err)
						return
					}
					if err := database.MarshalAndPut(txn, gDBI, []byte(id+".backup.enabled"), true); err != nil {
						lErr = fmt.Errorf("failed to marshal guild backup enabled for %s: %w", guild.ID, err)
						return
					}
					if err := database.MarshalAndPut(txn, gDBI, []byte(id+".favoriteChannelID"), ""); err != nil {
						lErr = fmt.Errorf("failed to marshal guild favorite channel ID for %s: %w", guild.ID, err)
						return
					}
					if err := database.MarshalAndPut(txn, gDBI, []byte(id+".uploadLimitMB"), 8); err != nil {
						lErr = fmt.Errorf("failed to marshal guild upload limit for %s: %w", guild.ID, err)
						return
					}
					if err := database.MarshalAndPut(txn, gDBI, []byte(id+".synctubeURL"), "https://youtu.be/PE0HpSknfw0?si=OBXhL6A7NTu44lPH"); err != nil {
						lErr = fmt.Errorf("failed to marshal guild synctube URL for %s: %w", guild.ID, err)
						return
					}
				} else {
					lErr = fmt.Errorf("failed to get guild name for %s: %w", guild.ID, err)
					return
				}
			} else {
				// update name if it has changed
				if name != guild.Name {
					xlog.Info(ctx, fmt.Sprintf("Guild %s name changed to %s, updating...", name, guild.Name))
					if err := database.MarshalAndPut(txn, gDBI, nameKey, guild.Name); err != nil {
						lErr = fmt.Errorf("failed to marshal guild name for %s: %w", guild.ID, err)
						return
					}
				}
			}
		})
		return lErr
	}); err != nil {
		xlog.Error(ctx, "failed to update database in onGuildsReady: %w", err)
		return
	}

	// TODO: register commands if needed
	if registerCommands {
		xlog.Info(ctx, "Registering commands...")
	}

	// TODO: start backup service

}

/*

func UpdateCommands() {
	blog.Info("Starting command registration...")
	registerGlobalCommands()
	registerGuildCommands()
	blog.Info("Command registration ended.")
}

func registerGlobalCommands() {
	// get global command creation data
	var globalCommands = []discord.ApplicationCommandCreate{}
	for _, command := range bot_commands.List {
		if command.IsGlobal {
			globalCommands = append(globalCommands, command.Data)
		}
	}
	blog.Debug(fmt.Sprintf("global commands being registered: %v", globalCommands))
	// register global commands
	if _, err := client.Client.Rest().SetGlobalCommands(client.Client.ApplicationID(), globalCommands); err != nil {
		blog.Error(fmt.Sprintf("error registering global commands: %s", err))
	}
}

func registerGuildCommands() {
	if utils.Config.Guilds == nil {
		blog.Warn("No guilds found in config.json")
		return
	}
	for id, guild := range utils.Config.Guilds {
		// get guild command creation data
		var guildCommands = []discord.ApplicationCommandCreate{}
		for name, command := range bot_commands.List {
			if !command.IsGlobal {
				// make sure command is not blacklisted
				if !utils.Contains(name, guild.CommandBlacklist) {
					guildCommands = append(guildCommands, command.Data)
				}
			}
		}
		blog.Debug(fmt.Sprintf("guild commands being registered for guild %s: %v", guild.Name, guildCommands))
		// register guild commands
		parsedGuildID, err := snowflake.Parse(id)
		if err != nil {
			blog.Error(fmt.Sprintf("error parsing guild ID %s: %s", id, err))
			continue
		}
		if _, err := client.Client.Rest().SetGuildCommands(client.Client.ApplicationID(), parsedGuildID, guildCommands); err != nil {
			blog.Error(fmt.Sprintf("error registering guild commands for guild %s: %s", guild.Name, err))
		}
	}
}

*/
