package listeners

import (
	"sprout/go/app"

	"github.com/disgoorg/disgo/events"
)

func OnGuildMessageCreate(a *app.App, event *events.GuildMessageCreate) {
	go func() {
		message, err := a.Client.Rest.GetMessage(event.Message.ChannelID, event.Message.ID)
		if err != nil {
			a.Log.Error("Failed to get message: ", err)
			return
		}
		if message.Author.Bot {
			return
		}

		a.Log.Info("Received message: ", message.Content)

		// TODO: Listener does per link, msg revive cmd new msg response to og with link to downloaded asset. If solo link attempt expand.

		/* old code
		if err := expand.Expand(ctx, message, false); err != nil {
			xlog.Error(ctx, "Failed to expand message: ", err)
		}
		*/
	}()
}
