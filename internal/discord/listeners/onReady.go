package listeners

import (
	"fmt"
	"sprout/internal/app"

	"github.com/disgoorg/disgo/events"
)

func OnReady(a *app.App, event *events.Ready) {
	fmt.Println("Halsey is now running. Press Ctrl+C to exit.")
	a.Log.Info("Halsey is now running.")
}
