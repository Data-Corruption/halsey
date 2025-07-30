package commands

import (
	"context"
	"encoding/json"
	"halsey/go/storage/config"
	"halsey/go/storage/database"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var setCommand = BotCommand{
	IsGlobal:     false,
	RequireAdmin: true,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "set",
		Description: "Set stuff.",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "favorites-channel",
				Description: "Sets the current channel as the favorites channel",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "bot-channel",
				Description: "Sets the current channel as the bot channel",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "bio",
				Description: "Sets halsey's bio picture",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "url",
						Description: "The URL of the bio picture",
						Required:    true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "bioh",
				Description: "Sets halsey's special picture",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "url",
						Description: "The URL of the special picture",
						Required:    true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "sync",
				Description: "Sets the url linked by /sync",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "url",
						Description: "The url to sync",
						Required:    true,
					},
				},
			},
		},
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
		data := event.SlashCommandInteractionData()

		// get db
		db := database.FromContext(ctx)
		if db == nil {
			xlog.Error(ctx, "database not found in context")
			return resMessageStr(ctx, event, "An internal error occurred while trying to get the database.", true)
		}

		switch *data.SubCommandName {
		case "favorites_channel":
			// marshal channel ID
			channelID, err := json.Marshal(event.Channel().ID().String())
			if err != nil {
				xlog.Errorf(ctx, "Error marshaling channel ID: %s", err)
				return resMessageStr(ctx, event, "An internal error occurred while trying to marshal the channel ID.", true)
			}
			// write to database
			key := []byte(event.GuildID().String() + ".favoriteChannelID")
			if err := db.Write(database.GuildsDBIName, key, channelID); err != nil {
				xlog.Errorf(ctx, "Error setting favorite channel ID in database: %s", err)
				return resMessageStr(ctx, event, "An internal error occurred while trying to set the favorite channel ID.", true)
			}
			return resMessageStr(ctx, event, "Favorite channel ID set successfully.", true)
		case "bot_channel":
			if err := config.Set(ctx, "botChannelID", event.Channel().ID().String()); err != nil {
				xlog.Errorf(ctx, "Error setting bot channel ID in config: %s", err)
				return resMessageStr(ctx, event, "An internal error occurred while trying to set the bot channel ID.", true)
			}
			return resMessageStr(ctx, event, "Bot channel ID set successfully.", true)
		case "bio":
			// write to config
			if err := config.Set(ctx, "bioURL", data.Options["url"].String()); err != nil {
				xlog.Errorf(ctx, "Error setting bio URL in config: %s", err)
				return resMessageStr(ctx, event, "An internal error occurred while trying to set the bio URL.", true)
			}
			return resMessageStr(ctx, event, "Bio URL set successfully.", true)
		case "bioh":
			// write to config
			if err := config.Set(ctx, "biohURL", data.Options["url"].String()); err != nil {
				xlog.Errorf(ctx, "Error setting bioh URL in config: %s", err)
				return resMessageStr(ctx, event, "An internal error occurred while trying to set the bioh URL.", true)
			}
			return resMessageStr(ctx, event, "Bioh URL set successfully.", true)
		case "sync":
			// write to database
			key := []byte(event.GuildID().String() + ".synctubeURL")
			if err := db.Write(database.GuildsDBIName, key, data.Options["url"].Value); err != nil {
				xlog.Errorf(ctx, "Error setting synctube URL in database: %s", err)
				return resMessageStr(ctx, event, "An internal error occurred while trying to set the synctube URL.", true)
			}
			return resMessageStr(ctx, event, "Synctube URL set successfully.", true)
		default:
			xlog.Errorf(ctx, "unknown subcommand %s, for command %s", *data.SubCommandName, data.CommandName())
			return resMessageStr(ctx, event, "An internal error occurred while trying to process the command.", true)
		}
	},
}

func init() {
	List["set"] = setCommand
}
