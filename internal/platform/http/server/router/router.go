package router

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sprout/internal/app"
	"sprout/internal/platform/http/server/router/css"
	"sprout/internal/platform/http/server/router/images"
	"sprout/internal/platform/http/server/router/js"
	"sprout/pkg/xcrypto"

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

	// serve embedded JS
	js.Init()
	r.Get(js.Path(), func(w http.ResponseWriter, r *http.Request) {
		data := js.Data()
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Write(data)
	})

	// serve embedded background images
	images.Init()
	r.Get("/bg/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		data := images.Data(name)
		if data == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/avif")
		w.Header().Set("Cache-Control", "public, max-age=86400") // 1 day cache
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
				http.Error(w, "hash required", http.StatusBadRequest)
				return
			}
			// validate hash
			if !xcrypto.IsSHA256LowerHex(hash) {
				http.Error(w, "invalid hash", http.StatusBadRequest)
				return
			}
			assetDir := filepath.Join(a.StorageDir, "assets")

			// Build the subdirectory path: ab/cd/
			subDir := filepath.Join(assetDir, hash[:2], hash[2:4])

			// Look for a file matching the hash with any extension
			pattern := filepath.Join(subDir, hash+".*")
			matches, err := filepath.Glob(pattern)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if len(matches) == 0 {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}

			// Serve the first match (there should only be one)
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(matches[0])))
			http.ServeFile(w, r, matches[0])
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
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https://cdn.discordapp.com; frame-ancestors 'self'")
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
