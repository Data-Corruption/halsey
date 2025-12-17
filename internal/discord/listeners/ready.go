package listeners

import (
	"fmt"
	"sprout/internal/app"

	"github.com/disgoorg/disgo/events"
)

func OnReady(a *app.App, event *events.Ready) {
	a.DiscordWG.Add(1) // track for graceful shutdown
	defer a.DiscordWG.Done()

	a.Chat.SetClient(a.Client)

	fmt.Println("Halsey is now running. Press Ctrl+C to exit.")
	a.Log.Info("Discord client is ready.")
}
