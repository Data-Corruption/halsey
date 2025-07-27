package listeners

import (
	"context"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/events"
)

func OnCommandInteraction(ctx context.Context, event *events.ApplicationCommandInteractionCreate) {
	xlog.Debugf(ctx, "Command interaction received: %s", event.Data.CommandName())
}
