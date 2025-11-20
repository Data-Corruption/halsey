package router

import (
	"net/http"
	"sprout/go/app"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/go-chi/chi/v5"
)

const HashParam = "h"

func New(a *app.App) *chi.Mux {
	r := chi.NewRouter()

	// inject logger into request context for xhttp.Error calls
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(xlog.IntoContext(r.Context(), a.Log)))
		})
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})

	r.Route("/download", func(s chi.Router) {
		s.Use(a.Net.DownloadAuth.Middleware(a.Config))

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

	// register settings routes
	settingsRoutes(a, r)

	return r
}
