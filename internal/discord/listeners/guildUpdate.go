package listeners

import (
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/events"
)

func OnGuildUpdate(a *app.App, event *events.GuildUpdate) {
	// not gonna bother limiting this

	if _, err := database.UpsertGuild(a.DB, event.Guild.ID, func(g *database.Guild) error {
		g.Name = event.Guild.Name
		g.PremiumTier = event.Guild.PremiumTier
		return nil
	}); err != nil {
		a.Log.Errorf("failed to upsert guild %s: %s", event.Guild.ID, err)
	} else {
		a.Log.Debugf("Guild %s (%s) updated successfully", event.Guild.Name, event.Guild.ID)
	}
}
