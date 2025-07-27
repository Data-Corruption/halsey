package listeners

import (
	"context"
	"halsey/go/disc/commands"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/events"
)

func OnCommandInteraction(ctx context.Context, event *events.ApplicationCommandInteractionCreate) {
	var cmdName string = event.Data.CommandName()
	xlog.Debugf(ctx, "Command interaction received: %s", cmdName)
	command, ok := commands.List[cmdName]
	if !ok {
		xlog.Warnf(ctx, "Unknown command: %s", cmdName)
		xlog.Debugf(ctx, "Commands map: %v", commands.List)
		return
	}
	command.Handler(ctx, event)
}
