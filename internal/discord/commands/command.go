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
	// Supports graceful shutdown. If you start any goroutines that outlive the handler, you must add them to `a.DiscordWG.Add(1)` and call `a.DiscordWG.Done()` when they are done.
	Handler func(a *app.App, event *events.ApplicationCommandInteractionCreate) error
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

// Response helpers

func createFollowupMessage(app *app.App, eventToken string, content string, ephemeral bool) error {
	_, err := app.Client.Rest.CreateFollowupMessage(app.Client.ApplicationID, eventToken, discord.NewMessageCreateBuilder().SetContent(content).SetEphemeral(ephemeral).Build())
	return err
}
