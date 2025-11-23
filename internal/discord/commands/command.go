package commands

import (
	"sprout/internal/app"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// Command struct for creating commands. See ping.go for an example.
type BotCommand struct {
	IsGlobal     bool
	RequireAdmin bool // if true, only admins can use this command
	FilterBots   bool // if true, bots cannot use this command
	Data         discord.ApplicationCommandCreate
	Handler      func(a *app.App, event *events.ApplicationCommandInteractionCreate) error
}

var Registry []BotCommand

func Get(name string) (BotCommand, bool) {
	for _, cmd := range Registry {
		if cmd.Data.CommandName() == name {
			return cmd, true
		}
	}
	return BotCommand{}, false
}

func register(cmd BotCommand) BotCommand {
	Registry = append(Registry, cmd)
	return cmd
}
