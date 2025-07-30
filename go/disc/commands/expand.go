package commands

import (
	"context"
	"halsey/go/disc"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var expandCommand = BotCommand{
	IsGlobal:     false,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.MessageCommandCreate{
		Name: "expand",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {

		data := event.MessageCommandInteractionData()
		message := data.TargetMessage()

		// if message author is the bot, do not expand
		if message.Author.ID == disc.Client.ID() {
			return resMessageStr(ctx, event, "I can't expand my own messages!", true)
		}

		return resMessageStr(ctx, event, "You expanded this message!", true)
	},
}

func init() {
	List["expand"] = expandCommand
}
