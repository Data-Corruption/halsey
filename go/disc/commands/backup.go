package commands

import (
	"context"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var backupCommand = BotCommand{
	IsGlobal: false,
	Data: discord.SlashCommandCreate{
		Name:        "backup",
		Description: "Manual server backup control.",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "download",
				Description: "Download the backup file.",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "start",
				Description: "Start or resume the backup.",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "stop",
				Description: "Stop the backup, resume with /backup start.",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "status",
				Description: "Get the status of the backup.",
			},
		},
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
		return nil
		/*
			data := event.SlashCommandInteractionData()

			// Reject bots / non-admins
			if event.User().Bot {
				utils.Respond(event, "Bots cannot use this command.", false)
				return
			}
			if !utils.Contains(event.User().ID.String(), utils.Config.AdminIdWhitelist) {
				utils.RespondTemp(event, "You do not have permission to use this command.", true, 5*time.Second)
				return
			}

			// Handle the subcommands
			switch *data.SubCommandName {
			case "download":
				token := backup.GenAuthToken()
				// get ip
				ip, err := utils.GetExternalIP()
				if err != nil {
					blog.Error(fmt.Sprintf("error getting external ip: %s", err))
					utils.Respond(event, "Error getting external ip.", true)
					return
				}
				if ip == "" {
					blog.Error("error getting external ip: empty string")
					utils.Respond(event, "Error getting external ip.", true)
					return
				}
				link := fmt.Sprintf("http://%s:%d/download-backup?auth=%s", ip, utils.Config.BackupPort, token)
				blog.Debugf("generated download link: %s", link)
				btnPayload := discord.NewMessageCreateBuilder().
					AddActionRow(discord.NewLinkButton("Download Server Backup", link)).
					SetContent("Expires in 15 minutes.").SetEphemeral(true).Build()
				err = event.CreateMessage(btnPayload)
				if err != nil {
					blog.Error(err.Error())
				}
			case "start":
				utils.Respond(event, "Starting backup, see bot channel for more info.", true)
				// Start the backup process
				if err := backup.Start(false); err != nil {
					blog.Error(fmt.Sprintf("error starting backup: %s", err))
					utils.MsgBotChannel("Error starting backup, check logs for more info.")
					return
				}
			case "stop":
				utils.Respond(event, "Stopping backup.", true)
				// Stop the backup process
				if err := backup.Stop(); err != nil {
					blog.Error(fmt.Sprintf("error stopping backup: %s", err))
					utils.MsgBotChannel("Error stopping backup, check logs for more info.")
					return
				}
			case "status":
				utils.Respond(event, "See bot channel for backup status.", true)
				// Get the status of the backup process
				if err := backup.Status(); err != nil {
					blog.Error(fmt.Sprintf("error getting backup status: %s", err))
					utils.MsgBotChannel("Error getting backup status.")
					return
				}
			default:
				blog.Error(fmt.Sprintf("unknown subcommand %s, for command %s", *data.SubCommandName, data.CommandName()))
			}
		*/
	},
}

func init() {
	List["backup"] = backupCommand
}
