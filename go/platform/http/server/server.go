package server

import (
	"fmt"
	"net/http"
	"sprout/go/app"
	"sprout/go/platform/database/config"
	"sprout/go/platform/sdnotify"

	"github.com/Data-Corruption/stdx/xhttp"
)

func New(app *app.App, handler http.Handler) error {
	// get http server related stuff from config
	port, err := config.Get[int](app.Config, "port")
	if err != nil {
		return fmt.Errorf("failed to get port from config: %w", err)
	}
	// create http server
	app.Net.Server, err = xhttp.NewServer(&xhttp.ServerConfig{
		Addr:    fmt.Sprintf(":%d", port),
		UseTLS:  false,
		Handler: handler,
		AfterListen: func() {
			// tell systemd we're ready
			status := fmt.Sprintf("Listening on %s", app.Net.Server.Addr())
			if err := sdnotify.Ready(status); err != nil {
				app.Log.Warnf("sd_notify READY failed: %v", err)
			}
		},
		OnShutdown: func() {
			// tell systemd we’re stopping
			if err := sdnotify.Stopping("Shutting down"); err != nil {
				app.Log.Debugf("sd_notify STOPPING failed: %v", err)
			}
			fmt.Println("shutting down, cleaning up resources ...")
		},
	})
	return err
}
