package listeners

import (
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/events"
)

func OnGuildMemberJoin(a *app.App, event *events.GuildMemberJoin) {
	a.DiscordWG.Add(1) // track for graceful shutdown
	defer a.DiscordWG.Done()

	// upsert user
	if _, err := database.UpsertUser(a.DB, event.Member.User.ID, func(user *database.User) error {
		user.Username = event.Member.User.Username
		user.AvatarURL = event.Member.User.AvatarURL()
		return nil
	}); err != nil {
		a.Log.Errorf("failed to upsert joining user: %s", err)
	}
}
