package router

import (
	"encoding/json"
	"net/http"
	"sprout/go/app"
	"sprout/go/platform/database/config"
	"sprout/go/platform/http/server/auth"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/go-chi/chi/v5"
)

const HashParam = "h"

type UpdateBody struct {
	RegisterCommands bool `json:"register_commands"`
}

func New(a *app.App) *chi.Mux {
	r := chi.NewRouter()

	// inject logger into request context for xhttp.Error calls
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(xlog.IntoContext(r.Context(), a.Log)))
		})
	})

	r.Route("/download", func(s chi.Router) {
		s.Use(a.Net.DownloadAuth.Middleware)

		s.Get("/backup", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("downloaded backed up server messages\n"))
		})

		s.Get("/a", func(w http.ResponseWriter, r *http.Request) {
			hash := r.URL.Query().Get(HashParam)
			if hash == "" {
				w.Write([]byte("baba booey\n"))
				return
			}
			w.Write([]byte("downloaded file with hash: " + hash + "\n"))
		})
	})

	r.Route("/settings", func(s chi.Router) {
		s.Use(a.Net.SettingsAuth.Middleware)

		// normal permissioned settings routes

		s.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("setting page\n"))
			// root settings page is a little special, will be diff based on admin or not
		})

		// admin only routes

		s.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// get session from context
				session, ok := auth.SessionFromContext(r.Context())
				if !ok {
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "internal server error", Err: auth.ErrNoSessionInContext})
					return
				}
				if isAdmin, err := auth.IsUserAdminByID(a.Config, session.UserID); err != nil {
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "internal server error", Err: err})
					return
				} else if !isAdmin {
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 403, Msg: "forbidden"})
					return
				}
				next.ServeHTTP(w, r)
			})
		})

		s.Get("/admin_thing", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("admin thing done\n"))
		})

		s.Post("/update", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

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
			w.Write([]byte("starting update...\n"))
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})
	return r
}

//lint:file-ignore SA1012 eat my ass staticcheck
