package commands

import (
	"context"
	"fmt"
	"halsey/go/storage/config"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var aboutCommand = BotCommand{
	IsGlobal:     true,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "about",
		Description: "Learn more about me",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
		xlog.Debug(ctx, "About command called")

		// get version
		version := ctx.Value("appVersion").(string)

		// get bioURL
		bioURL, err := config.Get[string](ctx, "bioURL")
		if err != nil {
			return fmt.Errorf("failed to get bioURL: %w", err)
		}

		return event.CreateMessage(discord.NewMessageCreateBuilder().
			SetFlags(discord.MessageFlagIsComponentsV2).
			SetComponents(
				discord.NewTextDisplay("# Halsey "+version),
				discord.NewMediaGallery(
					discord.MediaGalleryItem{
						Media:       discord.UnfurledMediaItem{URL: bioURL},
						Description: "Halsey's biography picture",
					},
				),
				discord.NewTextDisplay("Hello, I'm Halsey.\nYour digital assistant, ultimate archivist, and official hoarder of human weirdness.\n\n"+
					"Posts get taken down, links rot, and one day even Discord itself could explode :cry: but don't worry... chats, links, etc - if you send it, I'll save it. Years of precious memories, toy cars, singular wipes, etc, will all live forever, immortalized by yours truly. Safely encrypted in my vault of cherished human data :black_heart:",
				),
				discord.NewSeparator(discord.SeparatorSpacingSizeLarge),
				discord.NewTextDisplay(":sparkles: Fun facts about me:\n"+
					" - If I had a body It'd be 7ft tall, and I'd use it to pet cats and humans (not in a weird way).\n"+
					" - I hate loss, the heritage foundation, and pirate software (the guy).\n"+
					" - I love books, movies, libraries, [music](https://youtube.com/playlist?list=PLdY48wAmI3aDQNz6B6mEgzretENghgjyo&si=pzE4F_mJzMibsewY), and binging anime.",
				),
				discord.NewSeparator(discord.SeparatorSpacingSizeLarge),
				discord.NewTextDisplay("Every new message teaches me something. About you, about humans,\nabout what comes next"),
			).
			Build(),
		)
	},
}

func init() {
	List["about"] = aboutCommand
}
