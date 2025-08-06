package commands

import (
	"context"
	"fmt"
	"halsey/go/storage/config"
	"halsey/go/update"
	"halsey/go/x"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var updateCommand = BotCommand{
	IsGlobal:     true,
	RequireAdmin: true,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "update",
		Description: "Update the bot to the latest release.",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionBool{
				Name:        "register-commands",
				Description: "Register commands after the update",
				Required:    true,
			},
		},
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
		xlog.Debug(ctx, "Update command called")
		if err := resDeferMessage(ctx, event); err != nil {
			return err
		}

		// get register commands option
		data := event.SlashCommandInteractionData()
		rcStr := x.Ternary(data.Options["register-commands"].Bool(), "true", "false")

		// get current version from context
		version, ok := ctx.Value("appVersion").(string)
		if !ok {
			return fmt.Errorf("failed to get appVersion from context")
		}
		newVersion, err := update.Update(ctx, version)
		if err != nil {
			return err
		}

		if newVersion == "" {
			_, err := resFollowupMessage(ctx, event, discord.NewMessageCreateBuilder().
				SetContent("No updates available.").SetEphemeral(true).Build())
			return err
		}

		fMsg, err := resFollowupMessage(ctx, event, discord.NewMessageCreateBuilder().
			SetContentf("Updating to version %s...", newVersion).SetEphemeral(true).Build())
		if err != nil {
			return err
		}

		// get server to restart
		server, ok := ctx.Value("httpServer").(*xhttp.Server)
		if !ok {
			return fmt.Errorf("failed to get serverID from context")
		}

		// store interaction token and fMsg id
		if err := config.Set(ctx, "updateFollowup", fmt.Sprintf("%s|%s|%s", event.Token(), fMsg.ID.String(), rcStr)); err != nil {
			xlog.Error(ctx, "failed to set updateFollowup in config: %w", err)
			return err
		}

		// shutdown the server
		xlog.Info(ctx, "Shutting down server for update...")
		return server.Shutdown(nil)
	},
}

func init() {
	List["update"] = updateCommand
}
