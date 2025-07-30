package commands

import (
	"context"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var pingCommand = BotCommand{
	IsGlobal:     true,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "ping",
		Description: "Check if the bot is responsive.",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
		xlog.Debug(ctx, "Ping command called")
		return resMessage(ctx, event, discord.NewMessageCreateBuilder().SetContent("Pong!").Build())
	},
}

func init() {
	List["ping"] = pingCommand
}
