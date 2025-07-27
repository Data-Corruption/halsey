package commands

import (
	"context"
	"halsey/go/disc/respond"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var pingCommand = BotCommand{
	IsGlobal: true,
	Data: discord.SlashCommandCreate{
		Name:        "ping",
		Description: "Check if the bot is responsive.",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) {
		xlog.Debug(ctx, "Ping command called")
		respond.Normal(ctx, event, "Pong!", true)
	},
}

func init() {
	List["ping"] = pingCommand
}
