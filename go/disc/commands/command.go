package commands

import (
	"context"
	"halsey/go/disc"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

// Command struct for creating commands. See ping.go for an example.
type BotCommand struct {
	IsGlobal     bool
	RequireAdmin bool // if true, only admins can use this command
	FilterBots   bool // if true, bots cannot use this command
	Data         discord.ApplicationCommandCreate
	Handler      func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error
}

var List map[string]BotCommand = make(map[string]BotCommand)

// helpers

func resMessage(ctx context.Context, event *events.ApplicationCommandInteractionCreate, messageCreate discord.MessageCreate) error {
	return event.Respond(discord.InteractionResponseTypeCreateMessage, messageCreate)
}

func resMessageStr(ctx context.Context, event *events.ApplicationCommandInteractionCreate, msg string, ephemeral bool) error {
	return event.Respond(discord.InteractionResponseTypeCreateMessage, discord.NewMessageCreateBuilder().SetContent(msg).SetEphemeral(ephemeral).Build())
}

func resMessageLink(ctx context.Context, event *events.ApplicationCommandInteractionCreate, label, url string, ephemeral bool) error {
	return event.Respond(discord.InteractionResponseTypeCreateMessage, discord.NewMessageCreateBuilder().
		AddActionRow(discord.NewLinkButton(label, url)).
		SetEphemeral(ephemeral).
		Build())
}

func resDeferMessage(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
	return event.Respond(discord.InteractionResponseTypeDeferredCreateMessage, nil)
}

func resFollowupMessage(ctx context.Context, event *events.ApplicationCommandInteractionCreate, messageCreate discord.MessageCreate) error {
	_, err := disc.Client.Rest.CreateFollowupMessage(disc.Client.ApplicationID, event.Token(), messageCreate)
	return err
}
