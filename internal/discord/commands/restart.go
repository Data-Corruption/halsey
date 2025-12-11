package commands

import (
	"sprout/internal/app"
	"sprout/internal/discord/emojis"
	"sprout/internal/platform/database"
	"sprout/pkg/x"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var Restart = register(BotCommand{
	IsGlobal:     true,
	RequireAdmin: true,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "restart",
		Description: "Backup for restarting in case my web interface explodes",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionBool{
				Name:        "register-commands",
				Description: "Register commands after the restart",
				Required:    true,
			},
			discord.ApplicationCommandOptionBool{
				Name:        "update",
				Description: "Update during the restart if available",
				Required:    true,
			},
		},
	},
	Handler: func(a *app.App, event *events.ApplicationCommandInteractionCreate) error {
		event.DeferCreateMessage(true)

		// parse stuff
		data := event.SlashCommandInteractionData()
		updateBool := data.Options["update"].Bool()

		// if update arg is true, check if there is an update available
		var doUpdate bool
		if updateBool {
			if updateAvailable, err := a.CheckForUpdate(); err != nil {
				a.Log.Errorf("Error checking for updates: %s", err)
				a.Client.Rest.CreateFollowupMessage(a.Client.ApplicationID, event.Token(), discord.NewMessageCreateBuilder().
					SetContent("Error checking for updates. Please check the logs for more details.").
					SetEphemeral(true).
					Build())
				return err
			} else if updateAvailable {
				doUpdate = true
			}
		}

		time.Sleep(1 * time.Second) // smoother ux

		// send initial message
		actionLabel := x.Ternary(doUpdate, "Updating", "Restarting")
		uMsg, err := a.Client.Rest.CreateFollowupMessage(a.Client.ApplicationID, event.Token(), discord.NewMessageCreateBuilder().
			SetContentf("%s %s...", emojis.GetSpinnerEmoji(a), actionLabel).
			SetEphemeral(true).
			Build())
		if err != nil {
			a.Log.Errorf("Error sending update message: %s", err)
			return err
		}

		// set update context in config
		if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
			cfg.RestartCtx = database.RestartContext{
				RegisterCmds:  data.Options["register-commands"].Bool(),
				ListenCounter: 0,
				IToken:        event.Token(),
				MessageID:     uMsg.ID,
			}
			return nil
		}); err != nil {
			a.Log.Errorf("Error updating restart context in config: %s", err)
			a.Client.Rest.UpdateFollowupMessage(a.Client.ApplicationID, event.Token(), uMsg.ID, discord.NewMessageUpdateBuilder().
				SetContent("Error updating restart context in config. Please check the logs for more details.").
				Build())
			return err
		}

		// do the restart
		if doUpdate {
			if err := a.DetachUpdate(); err != nil {
				a.Log.Errorf("failed to detach update: %v", err)
			}
		} else {
			go a.Server.Shutdown()
		}
		return nil
	},
})
