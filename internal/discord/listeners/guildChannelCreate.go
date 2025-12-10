package listeners

import (
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/events"
)

func OnGuildChannelCreate(a *app.App, event *events.GuildChannelCreate) {
	// upsert channel
	if _, err := database.UpsertChannel(a.DB, event.Channel.ID(), func(c *database.Channel) error {
		c.GuildID = event.Channel.GuildID()
		c.Name = event.Channel.Name()
		c.Type = event.Channel.Type()
		return nil
	}); err != nil {
		a.Log.Errorf("failed to upsert channel %s: %s", event.Channel.ID(), err)
	}
}
