package commands

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/storage/config"
	"sync/atomic"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var busy atomic.Bool

var sendCommand = BotCommand{
	IsGlobal:     true,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "send",
		Description: "Ask Halsey to send you something.",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "nudes",
				Description: "Well... aren't you bold",
			},
		},
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
		if busy.Load() {
			return resMessageStr(ctx, event, "Hold on, I'm a little busy", true)
		}
		busy.Store(true)
		defer busy.Store(false)
		if err := resMessageStr(ctx, event, "lmao, you wish", true); err != nil {
			return fmt.Errorf("failed to respond: %w", err)
		}
		time.Sleep(20 * time.Second)

		biohURL, err := config.Get[string](ctx, "biohURL")
		if err != nil {
			return fmt.Errorf("failed to get biohURL: %w", err)
		}

		dmChannel, err := disc.Client.Rest.CreateDMChannel(event.User().ID)
		if err != nil {
			return fmt.Errorf("failed to create DM channel: %w", err)
		}

		if _, err := disc.Client.Rest.CreateMessage(dmChannel.ID(), discord.NewMessageCreateBuilder().
			SetFlags(discord.MessageFlagIsComponentsV2).
			SetComponents(
				discord.NewMediaGallery(
					discord.MediaGalleryItem{
						Media:       discord.UnfurledMediaItem{URL: biohURL},
						Description: "Halsey's biography picture",
					},
				),
				discord.NewTextDisplay(":black_heart:"),
			).
			Build(),
		); err != nil {
			xlog.Errorf(ctx, "Error sending nudes: %s", err)
		}

		xlog.Infof(ctx, "Sent nudes to user %s", event.User().ID)
		return nil
	},
}

func init() {
	List["send"] = sendCommand
}
