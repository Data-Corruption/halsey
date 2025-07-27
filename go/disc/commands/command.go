package commands

import (
	"context"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// Command struct for creating commands. See ping.go for an example.
type BotCommand struct {
	IsGlobal bool
	Data     discord.ApplicationCommandCreate
	Handler  func(ctx context.Context, event *events.ApplicationCommandInteractionCreate)
}

var List map[string]BotCommand = make(map[string]BotCommand)
