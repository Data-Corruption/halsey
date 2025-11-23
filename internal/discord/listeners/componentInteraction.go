package listeners

import (
	"sprout/internal/app"
	"sprout/internal/discord/components"
	"strings"

	"github.com/disgoorg/disgo/events"
)

func OnComponentInteraction(a *app.App, event *events.ComponentInteractionCreate) {
	go func() {
		// split event.Data.CustomID() on '.', prefix is what we switch on. Could also not have a second part.
		idParts := strings.Split(event.Data.CustomID(), ".")
		if len(idParts) < 1 {
			a.Log.Warnf("Component interaction with empty CustomID: %s", event.Data.CustomID())
			return
		}
		// get component
		component, found := components.Get(idParts[0])
		if !found {
			a.Log.Warnf("Unknown component interaction: %s", event.Data.CustomID())
			return
		}
		// call handler, passing in the rest of the idParts
		if err := component.Handler(a, event, idParts[1:]); err != nil {
			a.Log.Errorf("Error handling component interaction %s: %s", event.Data.CustomID(), err)
		}
	}()
}
