package listeners

import (
	"context"
	"halsey/go/disc"
	"halsey/go/disc/expand"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/events"
)

func OnGuildMessageCreate(ctx context.Context, event *events.GuildMessageCreate) {
	go func() {
		message, err := disc.Client.Rest.GetMessage(event.Message.ChannelID, event.Message.ID)
		if err != nil {
			xlog.Error(ctx, "Failed to get message: ", err)
			return
		}
		if message.Author.Bot {
			return
		}
		if err := expand.ExpandTest(ctx, message); err != nil {
			xlog.Error(ctx, "Failed to expand message: ", err)
		}
	}()
}
