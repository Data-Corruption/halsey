package commands

import (
	"fmt"
	"path/filepath"
	"sprout/internal/app"
	"sprout/internal/discord/externallinks"
	"sprout/internal/platform/database"
	"strings"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

var Download = register(BotCommand{
	IsGlobal:     false,
	RequireAdmin: false,
	FilterBots:   true,
	Data: discord.MessageCommandCreate{
		Name: "download",
	},
	Handler: func(a *app.App, event *events.ApplicationCommandInteractionCreate) error {
		if err := event.DeferCreateMessage(true); err != nil {
			return err
		}

		data := event.MessageCommandInteractionData()
		message := data.TargetMessage()

		links := externallinks.ExtractLinks(&message)
		if len(links) == 0 {
			// might be auto-expand output, search buttons
			links = externallinks.ExtractLinksFromButtons(&message)
			if len(links) == 0 {
				return createFollowupMessage(a, event.Token(), "No valid links found.", true)
			}
		}

		type Result struct {
			Link  externallinks.Link
			Asset *database.Asset
		}

		results := make([]Result, 0)
		for _, link := range links {
			asset, err := database.ViewAsset(a.DB, link.Url)
			if err != nil && !lmdb.IsNotFound(err) {
				return createFollowupMessage(a, event.Token(), "Failed to get asset: "+err.Error(), true)
			}
			if asset != nil {
				results = append(results, Result{
					Link:  link,
					Asset: asset,
				})
			}
		}

		if len(results) == 0 {
			return createFollowupMessage(a, event.Token(), "No downloaded media found.", true)
		}

		token, err := a.AuthManager.NewParamSession(a.DB, event.User().ID)
		if err != nil {
			return createFollowupMessage(a, event.Token(), "Failed to create session: "+err.Error(), true)
		}
		urlBase := fmt.Sprintf("%s/download/a?a=%s&h=", a.BaseURL, token)

		var msg strings.Builder
		fmt.Fprintf(&msg, "The following download links are valid for %s\n\n", a.AuthManager.TTL().String())

		for _, result := range results {
			hash := strings.TrimSuffix(filepath.Base(result.Asset.Path), filepath.Ext(result.Asset.Path))
			u := result.Link.Url
			if len(u) > 64 {
				u = u[:61] + "..."
			}
			fmt.Fprintf(&msg, "[Download](<%s>) `%s`\n", urlBase+hash, u)
		}

		return createFollowupMessage(a, event.Token(), msg.String(), true)
	},
})
