package commands

import (
	"fmt"
	"sprout/internal/app"
	"sprout/internal/platform/database"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

const banner = `
 __  __     ______     __         ______     ______     __  __    
/\ \_\ \   /\  __ \   /\ \       /\  ___\   /\  ___\   /\ \_\ \   
\ \  __ \  \ \  __ \  \ \ \____  \ \___  \  \ \  __\   \ \____ \  
 \ \_\ \_\  \ \_\ \_\  \ \_____\  \/\_____\  \ \_____\  \/\_____\ 
  \/_/\/_/   \/_/\/_/   \/_____/   \/_____/   \/_____/   \/_____/ 
                                                                  
`

var About = register(BotCommand{
	IsGlobal:     true,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "about",
		Description: "Learn more about me ;p",
	},
	Handler: func(a *app.App, event *events.ApplicationCommandInteractionCreate) error {
		// get configuration
		cfg, err := database.ViewConfig(a.DB)
		if err != nil {
			return fmt.Errorf("failed to get configuration from database: %w", err)
		}

		msgBuilder := discord.NewMessageCreateBuilder().SetFlags(discord.MessageFlagIsComponentsV2)
		msgBuilder.AddComponents(discord.NewTextDisplay("> " + a.Version + "\n```" + banner + "```"))
		if cfg.BioImageURL != "" {
			msgBuilder.AddComponents(
				discord.NewMediaGallery(
					discord.MediaGalleryItem{
						Media:       discord.UnfurledMediaItem{URL: cfg.BioImageURL},
						Description: "Halsey's biography picture",
					},
				),
			)
		}
		msgBuilder.AddComponents(
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
		)

		return event.CreateMessage(msgBuilder.Build())
	},
})
