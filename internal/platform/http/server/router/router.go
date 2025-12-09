package router

import (
	"net/http"
	"sprout/internal/app"
	"sprout/internal/platform/http/server/router/css"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/go-chi/chi/v5"
)

func New(a *app.App) *chi.Mux {
	r := chi.NewRouter()

	// inject logger into request context for xhttp.Error calls
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(xlog.IntoContext(r.Context(), a.Log)))
		})
	})

	// basic security hardening
	if a.Version != "vX.X.X" {
		r.Use(httpsRedirect)
	}
	r.Use(securityHeaders)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://github.com/Data-Corruption/halsey", http.StatusSeeOther)
	})

	// serve embedded CSS
	css.Init()
	r.Get(css.Path(), func(w http.ResponseWriter, r *http.Request) {
		data := css.Data()
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Write(data)
	})

	r.Route("/download", func(s chi.Router) {
		s.Use(a.AuthManager.Param(a.DB))

		s.Get("/backup", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("downloaded backed up server messages\n"))
		})

		s.Get("/a", func(w http.ResponseWriter, r *http.Request) {
			hash := r.URL.Query().Get("h")
			if hash == "" {
				w.Write([]byte("baba booey\n"))
				return
			}
			w.Write([]byte("downloaded file with hash: " + hash + "\n"))
		})
	})

	// exchange the out-of-band discord session for a cookie session and redirect to /settings page
	r.Handle("/login", a.AuthManager.Upgrade(a.DB, "/settings"))

	settingsRoutes(a, r)

	return r
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'self'")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

func httpsRedirect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-Proto") == "http" || (r.TLS == nil && r.Header.Get("X-Forwarded-Proto") == "") {
			if r.Host != "localhost" && r.Host != "127.0.0.1" && r.Host != "" {
				target := "https://" + r.Host + r.URL.RequestURI()
				http.Redirect(w, r, target, http.StatusSeeOther)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
