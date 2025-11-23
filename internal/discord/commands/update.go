package commands

import (
	"sprout/internal/app"
	"sprout/internal/discord/emojis"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var Update = register(BotCommand{
	IsGlobal:     true,
	RequireAdmin: true,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "update",
		Description: "Alternative update trigger in case of web UI issues.",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionBool{
				Name:        "register-commands",
				Description: "Register commands after the update",
				Required:    true,
			},
		},
	},
	Handler: func(a *app.App, event *events.ApplicationCommandInteractionCreate) error {
		if a.Version == "vX.X.X" {
			event.Respond(discord.InteractionResponseTypeCreateMessage, discord.NewMessageCreateBuilder().
				SetContent("This is a dev build, skipping update.").
				Build())
			return nil
		}
		event.Respond(discord.InteractionResponseTypeDeferredCreateMessage, nil)

		// parse stuff
		data := event.SlashCommandInteractionData()
		rcStr := data.Options["register-commands"].Bool()

		// do we really need to though?
		if upToDate, err := a.UpdateCheck(); err != nil {
			a.Log.Errorf("Error checking for updates: %s", err)
			a.Client.Rest.CreateFollowupMessage(a.Client.ApplicationID, event.Token(), discord.NewMessageCreateBuilder().
				SetContent("Error checking for updates. Please check the logs for more details.").
				SetEphemeral(true).
				Build())
			return err
		} else if upToDate {
			a.Client.Rest.CreateFollowupMessage(a.Client.ApplicationID, event.Token(), discord.NewMessageCreateBuilder().
				SetContent("I'm already up to date!").
				SetEphemeral(true).
				Build())
			return nil
		}

		// send initial update message
		uMsg, err := a.Client.Rest.CreateFollowupMessage(a.Client.ApplicationID, event.Token(), discord.NewMessageCreateBuilder().
			SetContentf("%s Updating... This may take a minute.", emojis.GetSpinnerEmoji(a)).
			SetEphemeral(true).
			Build())
		if err != nil {
			a.Log.Errorf("Error sending update message: %s", err)
		}

		// set update context in config
		rc := database.RestartContext{
			RegisterCmds: rcStr,
			IToken:       event.Token(),
			MessageID:    uMsg.ID,
		}
		if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
			cfg.RestartCtx = rc
			return nil
		}); err != nil {
			a.Log.Errorf("Error updating restart context in config: %s", err)
			a.Client.Rest.UpdateFollowupMessage(a.Client.ApplicationID, event.Token(), uMsg.ID, discord.NewMessageUpdateBuilder().
				SetContent("Error preparing for update. Please check the logs for more details.").
				Build())
			return err
		}

		// start update
		if err := a.Update(true); err != nil {
			a.Log.Errorf("Error during update: %s", err)
			a.Client.Rest.UpdateFollowupMessage(a.Client.ApplicationID, event.Token(), uMsg.ID, discord.NewMessageUpdateBuilder().
				SetContent("Update start failed, see logs for details.").
				Build())
			return err
		}

		return a.Net.Server.Shutdown(nil)
	},
})

//lint:file-ignore SA1012 eat my ass staticcheck
