package commands

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var thoughtsCommand = BotCommand{
	IsGlobal:     false,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.MessageCommandCreate{
		Name: "thoughts",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
		if err := resDeferMessage(ctx, event); err != nil {
			return fmt.Errorf("error deferring interaction: %s", err)
		}
		time.Sleep(2 * time.Second)
		_, err := resFollowupMessage(ctx, event, discord.NewMessageCreateBuilder().
			SetContent(cuteMessages[rand.Intn(len(cuteMessages))]).Build())
		if err != nil {
			return fmt.Errorf("error sending followup message: %s", err)
		}
		return nil
	},
}

func init() {
	List["thoughts"] = thoughtsCommand
}
