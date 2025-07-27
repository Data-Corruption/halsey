package respond

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/storage/config"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func Normal(ctx context.Context, event *events.ApplicationCommandInteractionCreate, content string, ephemeral bool) {
	err := event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(content).
		SetEphemeral(ephemeral).
		Build(),
	)
	if err != nil {
		xlog.Errorf(ctx, "Error responding to interaction: %s", err)
	}
}

func Temp(ctx context.Context, event *events.ApplicationCommandInteractionCreate, content string, ephemeral bool, duration time.Duration) {
	if err := event.CreateMessage(discord.NewMessageCreateBuilder().
		SetContent(content).
		SetEphemeral(ephemeral).
		Build(),
	); err != nil {
		xlog.Error(ctx, fmt.Sprintf("Error responding to interaction: %s", err))
		return
	}
	go func() {
		time.Sleep(duration)
		if err := disc.Client.Rest().DeleteInteractionResponse(disc.Client.ApplicationID(), event.Token()); err != nil {
			xlog.Errorf(ctx, "Error deleting interaction response: %s", err)
		}
	}()
}

func Link(ctx context.Context, event *events.ApplicationCommandInteractionCreate, label, url string, ephemeral bool) {
	xlog.Infof(ctx, "Responding with link: %s\n", url)
	err := event.CreateMessage(discord.NewMessageCreateBuilder().
		SetEphemeral(ephemeral).
		AddActionRow(discord.NewLinkButton(label, url)).
		Build(),
	)
	if err != nil {
		xlog.Errorf(ctx, "Error responding to interaction with link: %s", err)
	}
}

func BotChannel(ctx context.Context, msg string) {
	// get bot channel ID from config
	id, err := config.Get[string](ctx, "botChannelID")
	if err != nil {
		xlog.Errorf(ctx, "Error getting bot channel ID from config: %s", err)
		return
	}
	if id == "" {
		xlog.Error(ctx, "Bot channel ID not set in config")
		return
	}

	if Config.BotChannelID == "" {
		xlog.Error(ctx, "Bot channel not set")
		return
	}
	botChannelID, err := snowflake.Parse(Config.BotChannelID)
	if err != nil {
		xlog.Errorf(ctx, "Error parsing bot channel ID: %s", err)
		return
	}
	if _, err := disc.Client.Rest().CreateMessage(botChannelID, discord.NewMessageCreateBuilder().SetContent(msg).Build()); err != nil {
		xlog.Errorf(ctx, "Error sending message to bot channel: %s", err)
	}
}
