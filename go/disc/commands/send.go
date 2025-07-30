package commands

import (
	"context"
	"halsey/go/disc"
	"halsey/go/disc/respond"
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
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) {
		if busy.Load() {
			respond.Normal(ctx, event, "Hold on, I'm a little busy", true)
			return
		}
		busy.Store(true)
		defer busy.Store(false)
		respond.Temp(ctx, event, "lmao, you wish", true, 5*time.Second)
		time.Sleep(25 * time.Second)

		biohURL, err := config.Get[string](ctx, "biohURL")
		if err != nil {
			xlog.Errorf(ctx, "Error getting biohURL: %s", err)
			return
		}

		dmChannel, err := disc.Client.Rest.CreateDMChannel(event.User().ID)
		if err != nil {
			xlog.Errorf(ctx, "Error creating DM channel: %s", err)
			return
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
	},
}

func init() {
	List["send"] = sendCommand
}
