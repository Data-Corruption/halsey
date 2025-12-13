package listeners

import (
	"slices"
	"sprout/internal/app"
	"sprout/internal/discord/commands"
	"sprout/internal/platform/database"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func OnGuildsReady(a *app.App, event *events.GuildsReady, registerCommands bool) {
	a.DiscordWG.Add(1) // track for graceful shutdown
	defer a.DiscordWG.Done()

	// ensure all guilds are in the database / updated
	if a.Client.Caches.GuildCache().Len() == 0 {
		a.Log.Warn("guild cache is empty")
	}
	for guild := range a.Client.Caches.GuildCache().All() {
		if created, err := database.UpsertGuild(a.DB, guild.ID, func(g *database.Guild) error {
			g.Name = guild.Name
			g.PremiumTier = guild.PremiumTier
			return nil
		}); err != nil {
			a.Log.Errorf("failed to upsert guild %s: %s", guild.ID, err)
		} else if created {
			a.Log.Infof("New guild detected: %s (%s), adding to database...", guild.Name, guild.ID)
		} else {
			a.Log.Debugf("Guild %s (%s) registered successfully", guild.Name, guild.ID)
		}
	}

	// ensure all channels are in the database / updated
	if a.Client.Caches.ChannelCache().Len() == 0 {
		a.Log.Warn("channel cache is empty")
	}
	foundChannels := []snowflake.ID{}
	for channel := range a.Client.Caches.ChannelCache().All() {
		foundChannels = append(foundChannels, channel.ID())
		if created, err := database.UpsertChannel(a.DB, channel.ID(), func(c *database.Channel) error {
			c.GuildID = channel.GuildID()
			c.Name = channel.Name()
			c.Type = channel.Type()
			c.Position = channel.Position()
			if parentID := channel.ParentID(); parentID != nil {
				c.ParentID = *parentID
			} else {
				c.ParentID = snowflake.ID(0)
			}
			// since these are snowflake based, making a channel with the same name after deleting
			// isn't like remaking it. So we don't need to set deleted to false or have any
			// system for reintegrating deleted channels.
			return nil
		}); err != nil {
			a.Log.Errorf("failed to upsert channel %s: %s", channel.ID(), err)
		} else if created {
			a.Log.Infof("New channel detected: %s (%s), adding to database...", channel.Name(), channel.ID())
		} else {
			a.Log.Debugf("Channel %s (%s) registered successfully", channel.Name(), channel.ID())
		}
	}

	// set Deleted to true for channels in DB that are not in the cache
	if err := database.UpdateChannels(a.DB, func(id snowflake.ID, c *database.Channel) error {
		if !slices.Contains(foundChannels, id) {
			c.Deleted = true
		}
		return nil
	}); err != nil {
		a.Log.Errorf("failed to update channels in database: %s", err)
	}

	// ensure all users are in the database (fetch via REST since cache only has event-based members)
	// also track members per guild for the Members field
	foundUsers := []snowflake.ID{}
	for guild := range a.Client.Caches.GuildCache().All() {
		guildMembers := []snowflake.ID{} // members for this specific guild
		var after snowflake.ID
		for {
			members, err := a.Client.Rest.GetMembers(guild.ID, 1000, after)
			if err != nil {
				a.Log.Errorf("failed to fetch members for guild %s: %s", guild.ID, err)
				break
			}
			if len(members) == 0 {
				break
			}
			for _, member := range members {
				if member.User.Bot {
					continue
				}
				// Always add to guild members list
				guildMembers = append(guildMembers, member.User.ID)

				// Only upsert user if we haven't seen them yet (first time across all guilds)
				if !slices.Contains(foundUsers, member.User.ID) {
					foundUsers = append(foundUsers, member.User.ID)
					if created, err := database.UpsertUser(a.DB, member.User.ID, func(u *database.User) error {
						u.Username = member.User.Username
						u.AvatarURL = member.User.AvatarURL()
						return nil
					}); err != nil {
						a.Log.Errorf("failed to upsert user %s: %s", member.User.ID, err)
					} else if created {
						a.Log.Infof("New user detected: %s (%s), adding to database...", member.User.Username, member.User.ID)
					} else {
						a.Log.Debugf("User %s (%s) registered successfully", member.User.Username, member.User.ID)
					}
				}
			}
			after = members[len(members)-1].User.ID
			if len(members) < 1000 {
				break // no more pages
			}
			time.Sleep(100 * time.Millisecond) // rate limit courtesy
		}

		// Update the guild's Members field with the complete member list
		if _, err := database.UpsertGuild(a.DB, guild.ID, func(g *database.Guild) error {
			g.Members = guildMembers
			return nil
		}); err != nil {
			a.Log.Errorf("failed to update members for guild %s: %s", guild.ID, err)
		} else {
			a.Log.Debugf("Guild %s members synced: %d members", guild.Name, len(guildMembers))
		}
	}

	// get a copy of and then clear restart context
	var rCtx database.RestartContext
	if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
		rCtx = cfg.RestartCtx                      // copy current
		cfg.RestartCtx.RegisterCmds = false        // clear
		cfg.RestartCtx.IToken = ""                 // clear
		cfg.RestartCtx.MessageID = snowflake.ID(0) // clear
		return nil
	}); err != nil {
		a.Log.Errorf("failed to clear restartContext in database config: %s", err)
		return
	}

	// register commands if requested
	if rCtx.RegisterCmds || registerCommands {
		a.Log.Info("Registering commands...")
		registerCmds(a)
	}
	a.Log.Debugf("Commands: %v", commands.Registry)

	// update the /update interaction if exists
	uMsg := "Restarted successfully!"
	if rCtx.PreUpdateVersion != a.Version {
		uMsg = "Updated to version " + a.Version + " successfully!"
	}
	if rCtx.IToken != "" && rCtx.MessageID != 0 {
		a.Log.Infof("Following up on update interaction. Token: %s, MessageID: %s", rCtx.IToken, rCtx.MessageID.String())
		_, err := a.Client.Rest.UpdateFollowupMessage(a.Client.ApplicationID, rCtx.IToken, rCtx.MessageID, discord.NewMessageUpdateBuilder().
			SetContent(uMsg).Build())
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
