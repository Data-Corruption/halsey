package listeners

import (
	"slices"
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func OnGuildBan(a *app.App, event *events.GuildBan) {
	a.DiscordWG.Add(1) // track for graceful shutdown
	defer a.DiscordWG.Done()

	// skip bots
	if event.User.Bot {
		return
	}

	// remove member from guild's Members field
	if _, err := database.UpsertGuild(a.DB, event.GuildID, func(guild *database.Guild) error {
		guild.Members = slices.DeleteFunc(guild.Members, func(id snowflake.ID) bool {
			return id == event.User.ID
		})
		return nil
	}); err != nil {
		a.Log.Errorf("failed to remove member from guild: %s", err)
	}
}
