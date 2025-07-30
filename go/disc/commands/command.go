package commands

import (
	"context"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// Command struct for creating commands. See ping.go for an example.
type BotCommand struct {
	IsGlobal     bool
	RequireAdmin bool // if true, only admins can use this command
	FilterBots   bool // if true, bots cannot use this command
	Data         discord.ApplicationCommandCreate
	Handler      func(ctx context.Context, event *events.ApplicationCommandInteractionCreate)
}

var List map[string]BotCommand = make(map[string]BotCommand)
