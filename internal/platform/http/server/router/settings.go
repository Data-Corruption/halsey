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
	"sprout/internal/platform/http/server/router/images"
	"sprout/internal/platform/http/server/router/js"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/settings.html
var tmplFS embed.FS

var tmpl = template.Must(template.ParseFS(tmplFS, "templates/settings.html"))

// RestartBody is the body of POST /settings/restart requests.
type RestartBody struct {
	RegisterCommands bool `json:"register_commands"`
	Update           bool `json:"update"`
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
				"JS":              js.Path(),
				"Background":      images.Next(),
				"Favicon":         template.URL(`data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text x='50%' y='.9em' font-size='90' text-anchor='middle'>🖤</text></svg>`),
				"Title":           "Settings - Halsey",
				"Version":         a.Version,
				"UpdateAvailable": cfg.UpdateAvailable && (a.Version != "vX.X.X"),
				"User":            session.User,
				"BioImageURL":     cfg.BioImageURL,
				// Admin config fields
				"LogLevel":  cfg.LogLevel,
				"Port":      cfg.Port,
				"Host":      cfg.Host,
				"ProxyPort": cfg.ProxyPort,
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

		admin.Post("/restart", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			// parse body
			var body RestartBody
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

			// should we update?
			var doUpdate bool
			if body.Update && a.Version != "vX.X.X" {
				doUpdate = true
			}

			// update restart context in config
			if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
				cfg.RestartCtx.RegisterCmds = body.RegisterCommands
				cfg.RestartCtx.ListenCounter = 0
				return nil
			}); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update config", Err: err})
				return
			}

			w.WriteHeader(http.StatusAccepted)

			// do the restart
			if doUpdate {
				if err := a.DetachUpdate(); err != nil {
					a.Log.Errorf("failed to detach update: %v", err)
				}
			} else {
				go a.Server.Shutdown()
			}
		})

		admin.Get("/restart-status", func(w http.ResponseWriter, r *http.Request) {
			cfg, err := database.ViewConfig(a.DB)
			if err != nil {
				xhttp.Error(r.Context(), w, err)
				return
			}

			restarted := cfg.RestartCtx.ListenCounter > 0
			updated := cfg.RestartCtx.PreUpdateVersion != "" && cfg.RestartCtx.PreUpdateVersion != a.Version

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]bool{"restarted": restarted, "updated": updated}); err != nil {
				xhttp.Error(r.Context(), w, err)
			}
		})
	})
}
