package components

import (
	"fmt"
	"sprout/internal/app"
	"sprout/internal/discord/externallinks"
	"sprout/internal/platform/database"
	"sprout/internal/platform/download"
	"sprout/pkg/x"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/events"
)

var Download = register(BotComponent{
	ID: "download",
	Handler: func(a *app.App, event *events.ComponentInteractionCreate, idParts []string) error {
		// if user is not admin, return
		user, err := database.ViewUser(a.DB, event.User().ID)
		if err != nil {
			a.Log.Error("Failed to get user: ", err)
			return err
		}
		if !user.IsAdmin {
			event.CreateMessage(buildMsg("You do not have permission for this action."))
			return nil
		}

		// parse interaction ID parts
		if len(idParts) != 1 {
			event.CreateMessage(buildMsg("An error occurred."))
			return fmt.Errorf("download interaction without content copy message ID: %s", event.Data.CustomID())
		}
		confirm := x.Ternary(idParts[0] == "confirm", true, false)

		// delete message containing the component
		if err := a.Client.Rest.DeleteMessage(event.Message.ChannelID, event.Message.ID); err != nil {
			a.Log.Errorf("Error deleting message containing component: %s", err)
		}

		if !confirm {
			return nil
		}

		// TODO: download the big yt video

		// link if first field
		fields := strings.Fields(event.Message.Content)
		if len(fields) < 2 {
			event.CreateMessage(buildMsg("An error occurred."))
			return fmt.Errorf("download interaction without content copy message ID: %s", event.Data.CustomID())
		}
		link := fields[0]
		if download.ParseDomain(link) != download.DomainYouTube {
			event.CreateMessage(buildMsg("An error occurred."))
			return fmt.Errorf("download interaction without content copy message ID: %s", event.Data.CustomID())
		}

		// download
		var path string
		wg := &sync.WaitGroup{}
		wg.Add(1)
		a.YoutubeQueue.Enqueue(link, false, func() error {
			path, err = download.YtDLP(a.Context, link, 30*time.Minute)
			wg.Done()
			return err
		})
		wg.Wait()
		if err != nil {
			return fmt.Errorf("failed to download: %w", err)
		}

		// add asset
		if err := externallinks.AddAsset(a, link, path); err != nil {
			a.Log.Error("Failed to add asset: ", err)
			return err
		}

		return nil
	},
})
