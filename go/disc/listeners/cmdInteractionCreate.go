package listeners

import (
	"context"
	"halsey/go/disc/commands"
	"halsey/go/disc/respond"
	"halsey/go/storage/config"
	"slices"

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
	// bot check
	if command.FilterBots && event.User().Bot {
		respond.Normal(ctx, event, "Bots cannot use this command.", false)
		return
	}
	// admin check
	if command.RequireAdmin {
		// get admin list from config
		adminUserIDs, err := config.Get[[]string](ctx, "adminUserIDs")
		if err != nil {
			xlog.Errorf(ctx, "Error getting admin list from config: %s", err)
			return
		}
		userID := event.User().ID.String()
		if !slices.Contains(adminUserIDs, userID) {
			xlog.Warnf(ctx, "User %s is not an admin", userID)
			respond.Normal(ctx, event, "You do not have permission to use this command.", true)
			return
		}
	}
	go command.Handler(ctx, event)
}
