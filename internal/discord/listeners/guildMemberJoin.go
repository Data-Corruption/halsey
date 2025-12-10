package listeners

import (
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/events"
)

func OnGuildMemberJoin(a *app.App, event *events.GuildMemberJoin) {
	// upsert user
	if _, err := database.UpsertUser(a.DB, event.Member.User.ID, func(user *database.User) error {
		user.Username = event.Member.User.Username
		user.AvatarURL = event.Member.User.AvatarURL()
		user.BannerURL = event.Member.User.BannerURL()
		return nil
	}); err != nil {
		a.Log.Errorf("failed to upsert joining user: %s", err)
	}
}
