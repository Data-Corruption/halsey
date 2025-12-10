package listeners

import (
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/events"
)

func OnGuildChannelDelete(a *app.App, event *events.GuildChannelDelete) {
	a.DiscordWG.Add(1) // track for graceful shutdown
	defer a.DiscordWG.Done()

	// set deleted to true for channel in DB
	if _, err := database.UpsertChannel(a.DB, event.ChannelID, func(c *database.Channel) error {
		c.Deleted = true
		return nil
	}); err != nil {
		a.Log.Errorf("failed to update channel %s in database: %s", event.ChannelID, err)
	}
}
