package commands

import (
	"sprout/go/app"
	"sprout/go/discord/emojis"
	"sprout/go/platform/database/config"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var Update = register(BotCommand{
	IsGlobal:     true,
	RequireAdmin: true,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "update",
		Description: "Backup update trigger in case of UI issues.",
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
		rc := config.RestartContext{
			RegisterCmds: rcStr,
			IToken:       event.Token(),
			MessageID:    uMsg.ID,
		}
		if err := config.Set(a.Config, "restartContext", rc); err != nil {
			a.Log.Errorf("failed to set restartContext in config: %s", err)
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
