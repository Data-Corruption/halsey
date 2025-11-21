package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sprout/go/app"
	"sprout/go/platform/database/config"
	"sprout/go/platform/http/server/auth"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/go-chi/chi/v5"
)

type UpdateBody struct {
	RegisterCommands bool `json:"register_commands"`
}

func settingsRoutes(a *app.App, r *chi.Mux) {
	r.Route("/settings", func(s chi.Router) {
		s.Use(a.Net.SettingsAuth.Middleware(a.Config))

		s.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("setting page\n"))
			// root settings page is a little special, will be diff based on admin or not
		})

		// normal permissioned settings routes

		// admin only routes.
		adminSettingsRoutes(a, s)

	})
}

func adminSettingsRoutes(a *app.App, r chi.Router) {
	r.Group(func(admin chi.Router) {
		admin.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// get session from context
				session, ok := auth.SessionFromContext(r.Context())
				if !ok {
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "internal server error", Err: auth.ErrNoSessionInContext})
					return
				}
				if !session.IsAdmin {
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 403, Msg: "forbidden", Err: fmt.Errorf("user %d is not an admin", session.UserID)})
					return
				}
				next.ServeHTTP(w, r)
			})
		})

		admin.Get("/admin_thing", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("admin thing done\n"))
		})

		admin.Post("/update", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			if a.Version == "vX.X.X" {
				w.Write([]byte("This is a dev build, skipping update.\n")) // replace this standardized res later
				return
			}

			// do we really need to though?
			if upToDate, err := a.UpdateCheck(); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "error checking for updates", Err: err})
				return
			} else if upToDate {
				w.Write([]byte("I'm already up to date!\n")) // replace this standardized res later
				return
			}

			// parse body
			var body UpdateBody
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields() // surfaces unexpected input early
			if err := dec.Decode(&body); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: err})
				return
			}
			if dec.More() {
				http.Error(w, "invalid JSON: trailing data", http.StatusBadRequest)
				return
			}

			// set update context in config
			rc := config.RestartContext{RegisterCmds: body.RegisterCommands}
			if err := config.Set(a.Config, "restartContext", &rc); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to set update context in config", Err: err})
				return
			}

			// start update
			if err := a.Update(true); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to start update", Err: err})
				return
			}
			a.Net.Server.Shutdown(nil)
			w.Write([]byte("starting update...\n")) // replace this standardized res later
		})
	})
}

//lint:file-ignore SA1012 eat my ass staticcheck
