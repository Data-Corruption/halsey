package commands

import (
	"context"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var settingsCommand = BotCommand{
	IsGlobal: false,
	Data: discord.SlashCommandCreate{
		Name:        "settings",
		Description: "Set various settings.",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "set_bookmark_channel",
				Description: "Sets the current channel as the bookmark channel",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "set_bot_channel",
				Description: "Sets the current channel as the bot channel",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "set_sync",
				Description: "Sets the url linked by /sync",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "url",
						Description: "The url to sync",
						Required:    true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "toggle_expand",
				Description: "Toggle automatic expansion of links",
			},
		},
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) {
		/*
			data := event.SlashCommandInteractionData()

			// Get a copy of the guild's config
			guildID := event.GuildID().String()
			guildConfig, exists := utils.Config.Guilds[guildID]
			if !exists {
				utils.RespondTemp(event, "This guild is not configured.", true, 5*time.Second)
				return
			}

			// Handle non admin commands

			if event.User().Bot {
				utils.Respond(event, "Bots cannot use this command.", false)
				return
			}

			switch *data.SubCommandName {
			case "toggle_expand":
				if utils.Contains(event.User().ID.String(), utils.Config.ExpandBlacklist) {
					utils.Config.ExpandBlacklist = utils.Remove(event.User().ID.String(), utils.Config.ExpandBlacklist)
					utils.RespondTemp(event, "Expand enabled.", true, 5*time.Second)
				} else {
					utils.Config.ExpandBlacklist = append(utils.Config.ExpandBlacklist, event.User().ID.String())
					utils.RespondTemp(event, "Expand disabled.", true, 5*time.Second)
				}
				utils.Config.Save()
				return
			}

			// Handle admin commands

			if !utils.Contains(event.User().ID.String(), utils.Config.AdminIdWhitelist) {
				utils.RespondTemp(event, "You do not have permission to use this command.", true, 5*time.Second)
				return
			}

			switch *data.SubCommandName {
			case "set_bookmark_channel":
				channelID := event.Channel().ID().String()
				guildConfig.BookmarkChannelID = channelID  // modify the copy
				utils.Config.Guilds[guildID] = guildConfig // put the copy back in the config
				utils.Config.Save()
				utils.RespondTemp(event, fmt.Sprintf("Set bookmark channel to <#%s>", channelID), true, 5*time.Second)
			case "set_bot_channel":
				channelID := event.Channel().ID().String()
				utils.Config.BotChannelID = channelID
				utils.Config.Save()
				utils.RespondTemp(event, fmt.Sprintf("Set bot channel to <#%s>", channelID), true, 5*time.Second)
			case "set_sync":
				var url string
				if err := json.Unmarshal(data.Options["url"].Value, &url); err != nil {
					blog.Error(fmt.Sprintf("Error unmarshalling url: %s", err))
					utils.RespondTemp(event, "An error occurred", true, 5*time.Second)
					return
				}
				guildConfig.SyncTubeURL = url              // modify the copy
				utils.Config.Guilds[guildID] = guildConfig // put the copy back in the config
				utils.Config.Save()
				utils.RespondTemp(event, fmt.Sprintf("Set SyncTubeURL to %s", url), true, 5*time.Second)
			default:
				blog.Error(fmt.Sprintf("unknown subcommand %s, for command %s", *data.SubCommandName, data.CommandName()))
			}

		*/
	},
}

func init() {
	List["settings"] = settingsCommand
}
