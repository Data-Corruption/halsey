package listeners

import (
	"fmt"
	"sprout/internal/app"
	"sprout/internal/discord/commands"
	"sprout/internal/platform/database"
	"time"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func OnGuildsReady(a *app.App, event *events.GuildsReady, rcFlag bool) {
	// get guild DBI
	gDBI, ok := a.DB.GetDBis()[database.GuildsDBIName]
	if !ok {
		a.Log.Error("failed to get guilds DBI from app DB")
		return
	}

	// update guilds in database
	if err := a.DB.Update(func(txn *lmdb.Txn) error {
		for guild := range a.Client.Caches.GuildCache().All() {
			// get key
			id := guild.ID.String()
			if id == "" {
				a.Log.Errorf("guild ID is empty, skipping: %v", guild)
				continue
			}
			key := []byte(id)

			// get guild from database, create if not exists
			var dbGuild database.Guild
			err := database.TxnGetAndUnmarshal(txn, gDBI, key, &dbGuild)
			if err != nil {
				if lmdb.IsNotFound(err) {
					// write guild to database with default values
					a.Log.Info(fmt.Sprintf("New guild detected: %s (%s), adding to database...", guild.Name, guild.ID))
					newGuild := database.Guild{
						Name:         guild.Name,
						FavChannelID: snowflake.ID(0),
						SynctubeURL:  "https://youtu.be/PE0HpSknfw0?si=OBXhL6A7NTu44lPH",
						PremiumTier:  int(guild.PremiumTier),
						// backup defaults to not enabled/empty runID
					}
					if err := database.TxnMarshalAndPut(txn, gDBI, key, &newGuild); err != nil {
						return fmt.Errorf("failed to add new guild to database: %w", err)
					}
					continue
				} else {
					return fmt.Errorf("failed to get guild for %s: %w", guild.ID, err)
				}
			}

			// update name if it has changed
			if dbGuild.Name != guild.Name {
				a.Log.Info(fmt.Sprintf("Guild %s name changed to %s, updating...", dbGuild.Name, guild.Name))
				dbGuild.Name = guild.Name
			}

			// update premium tier if it has changed
			if dbGuild.PremiumTier != int(guild.PremiumTier) {
				a.Log.Info(fmt.Sprintf("Guild %s premium tier changed to %d, updating...", dbGuild.Name, guild.PremiumTier))
				dbGuild.PremiumTier = int(guild.PremiumTier)
			}

			// write updated guild back to database
			if err := database.TxnMarshalAndPut(txn, gDBI, key, &dbGuild); err != nil {
				return fmt.Errorf("failed to update guild %s in database: %w", guild.ID, err)
			}
		}
		return nil
	}); err != nil {
		a.Log.Errorf("issue updating guilds in database: %s", err)
		return
	}

	// get a copy of and then clear restart context
	var rCtx database.RestartContext
	if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
		rCtx = cfg.RestartCtx                      // copy current
		cfg.RestartCtx = database.RestartContext{} // clear
		return nil
	}); err != nil {
		a.Log.Errorf("failed to clear restartContext in database config: %s", err)
		return
	}

	// register commands if requested
	if rCtx.RegisterCmds || rcFlag {
		a.Log.Info("Registering commands...")
		registerCmds(a)
	}
	a.Log.Debugf("Commands: %v", commands.Registry)

	// update the /update interaction if exists
	if rCtx.IToken != "" && rCtx.MessageID != 0 {
		a.Log.Infof("Following up on update interaction. Token: %s, MessageID: %s", rCtx.IToken, rCtx.MessageID.String())
		_, err := a.Client.Rest.UpdateFollowupMessage(a.Client.ApplicationID, rCtx.IToken, rCtx.MessageID, discord.NewMessageUpdateBuilder().
			SetContentf("Updated to version %s successfully!", a.Version).Build())
		if err != nil {
			a.Log.Errorf("failed to update followup message: %s", err)
			// don't return here since err could be due to user deleting the message or something
		}
	}

	// TODO: start backup service when i build that

}

func registerCmds(a *app.App) {
	// get command creation data
	var globalCommands = []discord.ApplicationCommandCreate{}
	var guildCommands = []discord.ApplicationCommandCreate{}
	for _, command := range commands.Registry {
		if command.IsGlobal {
			globalCommands = append(globalCommands, command.Data)
		} else {
			guildCommands = append(guildCommands, command.Data)
		}
	}
	// register global commands
	a.Log.Debugf("global commands being registered: %v", globalCommands)
	if _, err := a.Client.Rest.SetGlobalCommands(a.Client.ApplicationID, globalCommands); err != nil {
		a.Log.Errorf("error registering global commands: %s", err)
	}
	// register guild commands
	a.Log.Debugf("guild commands being registered: %v", guildCommands)
	for guild := range a.Client.Caches.GuildCache().All() {
		if _, err := a.Client.Rest.SetGuildCommands(a.Client.ApplicationID, guild.ID, guildCommands); err != nil {
			a.Log.Errorf("error registering guild commands for guild %s: %s", guild.Name, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
