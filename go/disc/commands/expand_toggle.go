package commands

import (
	"context"
	"halsey/go/storage/config"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var expandToggleCommand = BotCommand{
	IsGlobal:     true,
	RequireAdmin: true,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "toggle-expand",
		Description: "Toggle link expansion settings.",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionString{
				Name:        "platform",
				Description: "The platform to toggle expansion for",
				Required:    true,
				Choices: []discord.ApplicationCommandOptionChoiceString{
					{Name: "Instagram", Value: "Instagram"},
					{Name: "Reddit", Value: "Reddit"},
					{Name: "Twitter", Value: "Twitter"},
					{Name: "YouTube", Value: "YouTube"},
				},
			},
		},
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
		// get platform option
		data := event.SlashCommandInteractionData()
		platform := data.Options["platform"].String()
		key := "expand" + platform
		// toggle the expansion setting for the specified platform
		current, err := config.Get[bool](ctx, key)
		if err != nil {
			return err
		}
		return config.Set(ctx, key, !current)
	},
}

func init() {
	List["toggle-expand"] = expandToggleCommand
}
