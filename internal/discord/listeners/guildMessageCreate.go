package listeners

import (
	"sprout/internal/app"

	"github.com/disgoorg/disgo/events"
)

func OnGuildMessageCreate(a *app.App, event *events.GuildMessageCreate) {
	// acquire semaphore
	select {
	case a.DiscordEventLimiter <- struct{}{}:
	default:
		a.Log.Warn("Event limiter reached, dropping guild message create")
		return
	}

	go func() {
		defer func() { <-a.DiscordEventLimiter }()

		if event.Message.Author.Bot {
			return
		}

		a.Log.Info("Received message: ", event.Message.Content)

		// TODO: Listener does per link, msg revive cmd new msg response to og with link to downloaded asset. If solo link attempt expand.

		/* old code
		if err := expand.Expand(ctx, message, false); err != nil {
			xlog.Error(ctx, "Failed to expand message: ", err)
		}
		*/
	}()
}
