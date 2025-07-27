package listeners

import (
	"context"
	"fmt"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/events"
)

func OnGuildMessageCreate(ctx context.Context, event *events.GuildMessageCreate) {
	xlog.Debug(ctx, fmt.Sprintf("Message created in guild %s", event.GuildID))
}
