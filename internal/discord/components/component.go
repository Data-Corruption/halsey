package components

import (
	"sprout/internal/app"

	"github.com/disgoorg/disgo/events"
)

// Component struct for components. See removeFavorite.go for an example.
type BotComponent struct {
	ID      string // custom ID prefix to identify the component
	Handler func(a *app.App, event *events.ComponentInteractionCreate, idParts []string) error
}

var Registry []BotComponent

func Get(id string) (BotComponent, bool) {
	for _, component := range Registry {
		if component.ID == id {
			return component, true
		}
	}
	return BotComponent{}, false
}

func register(component BotComponent) BotComponent {
	Registry = append(Registry, component)
	return component
}
