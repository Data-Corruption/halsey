package commands

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/disc/expand"
	"time"

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
		if err := resDeferMessage(ctx, event); err != nil {
			return fmt.Errorf("error deferring interaction: %s", err)
		}
		time.Sleep(2 * time.Second)
		if err := disc.Client.Rest.DeleteInteractionResponse(disc.Client.ApplicationID, event.Token()); err != nil {
			return err
		}
		msg := event.MessageCommandInteractionData().TargetMessage()
		return expand.ExpandTest(ctx, &msg)
	},
}

func init() {
	List["expand"] = expandCommand
}
