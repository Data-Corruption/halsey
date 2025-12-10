package router

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sprout/internal/app"
	"sprout/internal/platform/auth"
	"sprout/internal/platform/database"
	"sprout/internal/platform/http/server/router/css"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/settings.html
var tmplFS embed.FS

var tmpl = template.Must(template.ParseFS(tmplFS, "templates/settings.html"))

// UpdateRestartBody is the body of POST /settings/update and /settings/restart requests.
type UpdateRestartBody struct {
	RegisterCommands bool `json:"register_commands"`
}

func settingsRoutes(a *app.App, r *chi.Mux) {
	r.Route("/settings", func(s chi.Router) {
		s.Use(a.AuthManager.Cookie(a.DB))

		s.Get("/", func(w http.ResponseWriter, r *http.Request) {
			cfg, err := database.ViewConfig(a.DB)
			if err != nil {
				xhttp.Error(r.Context(), w, err)
				return
			}

			// get session from context
			session, ok := auth.SessionFromContext(r.Context())
			if !ok {
				xhttp.Error(r.Context(), w, auth.ErrNoSessionInContext)
				return
			}

			data := map[string]any{
				"CSS":             css.Path(),
				"Favicon":         template.URL(`data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text x='50%' y='.9em' font-size='90' text-anchor='middle'>🖤</text></svg>`),
				"Title":           "Settings - Halsey",
				"Version":         a.Version,
				"UpdateAvailable": cfg.UpdateAvailable && (a.Version != "vX.X.X"),
				"User":            session.User,
			}
			if err := tmpl.Execute(w, data); err != nil {
				xhttp.Error(r.Context(), w, err)
				return
			}
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
				if !session.User.IsAdmin {
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 403, Msg: "forbidden", Err: fmt.Errorf("user %d is not an admin", session.UserID)})
					return
				}
				next.ServeHTTP(w, r)
			})
		})

		admin.Post("/update", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()
			if a.Version == "vX.X.X" {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: fmt.Errorf("this is a dev build, skipping update")})
				return
			}

			// parse body
			var body UpdateRestartBody
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields() // surfaces unexpected input early
			if err := dec.Decode(&body); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: err})
				return
			}
			if dec.More() {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: fmt.Errorf("invalid JSON: trailing data")})
				return
			}

			// update restart context in config
			if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
				cfg.RestartCtx.RegisterCmds = body.RegisterCommands
				return nil
			}); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update config", Err: err})
				return
			}

			w.WriteHeader(http.StatusAccepted)

			if err := a.DetachUpdate(); err != nil {
				a.Log.Errorf("failed to detach update: %v", err)
			}
		})

		admin.Get("/update-status", func(w http.ResponseWriter, r *http.Request) {
			cfg, err := database.ViewConfig(a.DB)
			if err != nil {
				xhttp.Error(r.Context(), w, err)
				return
			}

			updating := (cfg.UpdateFollowup != "") && (cfg.UpdateFollowup == a.Version) && (cfg.ListenCounter > 0)
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]bool{"updating": updating}); err != nil {
				xhttp.Error(r.Context(), w, err)
			}
		})

		/*
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

				// update restart context in config
				if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
					cfg.RestartCtx.RegisterCmds = body.RegisterCommands
					return nil
				}); err != nil {
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update config", Err: err})
					return
				}

				// start update
				if err := a.Update(true); err != nil {
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to start update", Err: err})
					return
				}
				a.Server.Shutdown()
				w.Write([]byte("starting update...\n")) // replace this standardized res later
			})
		*/
	})
}
