// Package auth provides a simple authentication model for HTTPS that bootstraps
// out-of-band via Discord. This is secure enough for the use case.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sprout/internal/platform/database"
	"sync"
	"time"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/lmdb-go/wrap"
	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/disgoorg/snowflake/v2"
	"golang.org/x/time/rate"
)

const (
	ParamName        = "a"
	CookieName       = "session"
	DefaultTTL       = 10 * time.Minute
	DefaultRateLimit = 1 * time.Second
	DefaultRateBurst = 5
)

var (
	ErrUninitialized      = errors.New("auth manager not initialized")
	ErrSessionCollision   = errors.New("generated session id already exists")
	ErrNoSessionInContext = errors.New("no session in context")
)

type Manager struct {
	// token -> process scoped param based session.
	// These are also used to create cookie based sessions that persist across restarts.
	paramSessions map[string]database.Session

	ttl   time.Duration
	limit *rate.Limiter
	mu    sync.Mutex
	init  bool
}

// New creates a new auth manager. Nil arguments use defaults.
func New(ttl *time.Duration, limiter *rate.Limiter) *Manager {
	m := &Manager{
		paramSessions: make(map[string]database.Session),
		ttl:           DefaultTTL,
		limit:         rate.NewLimiter(rate.Every(DefaultRateLimit), DefaultRateBurst),
		init:          true,
	}

	if ttl != nil {
		m.ttl = *ttl
	}
	if limiter != nil {
		m.limit = limiter
	}

	return m
}

func (m *Manager) TTL() time.Duration {
	return m.ttl
}

// NewParamSession generates a new param session for the given user.
// It errors if the generated token collides with an existing one.
// It also cleans up expired param sessions.
// Example URL with token and other params: /download?a=abcd1234&h=hash
func (m *Manager) NewParamSession(db *wrap.DB, userID snowflake.ID) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.init {
		return "", ErrUninitialized
	}

	// gen token
	token, err := genRandomString(16)
	if err != nil {
		return "", err
	}

	// check collision
	if _, exists := m.paramSessions[token]; exists {
		return "", ErrSessionCollision
	}

	// get user
	user, err := database.ViewUser(db, userID)
	if err != nil {
		return "", err
	}

	// create session
	expiration := time.Now().Add(m.ttl)
	m.paramSessions[token] = database.Session{
		UserID:     userID,
		User:       *user,
		Expiration: expiration,
	}

	// clean up expired sessions
	now := time.Now()
	for t, s := range m.paramSessions {
		if s.Expiration.Before(now) {
			delete(m.paramSessions, t)
		}
	}

	return token, nil
}

// Param is middleware that rejects requests without a valid param session.
// Also embeds the session in the request context for downstream handlers to use.
func (m *Manager) Param(db *wrap.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get(ParamName)
			if token == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// ensure initialized / get session
			m.mu.Lock()
			if !m.init {
				m.mu.Unlock()
				xhttp.Error(r.Context(), w, ErrUninitialized)
				return
			}
			session, exists := m.paramSessions[token]
			m.mu.Unlock()

			// handle missing / expired session
			if !exists || time.Now().After(session.Expiration) {
				// rate limit
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				if err := m.limit.Wait(ctx); err != nil {
					// could be err, timeout, or burst exceeded
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 429, Msg: "too many requests, try again later", Err: err})
					return
				}
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// get user info
			user, err := database.ViewUser(db, session.UserID)
			if err != nil || user == nil {
				xhttp.Error(r.Context(), w, err)
				return
			}
			session.User = *user

			// put session into context
			ctx := ContextWithSession(r.Context(), session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Upgrade upgrades a param session to a cookie session. It's placed on a route with no other
// auth middleware, e.g., something like /login which redirects to /settings.
func (m *Manager) Upgrade(db *wrap.DB, redirectTo string) http.Handler {
	return m.Param(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get session from context (set by Param middleware)
		session, ok := SessionFromContext(r.Context())
		if !ok {
			xhttp.Error(r.Context(), w, ErrNoSessionInContext)
			return
		}

		// generate new token for cookie session
		token, err := genRandomString(16)
		if err != nil {
			xhttp.Error(r.Context(), w, err)
			return
		}

		// store session in database
		err = db.Update(func(txn *lmdb.Txn) error {
			dbi := db.GetDBis()[database.SessionsDBIName]
			// check for collision
			var existingSess database.Session
			if err := database.TxnGetAndUnmarshal(txn, dbi, []byte(token), &existingSess); err == nil {
				return ErrSessionCollision
			}
			// store session
			return database.TxnMarshalAndPut(txn, dbi, []byte(token), session)
		})
		if err != nil {
			xhttp.Error(r.Context(), w, err)
			return
		}

		// set cookie
		http.SetCookie(w, &http.Cookie{
			Name:     CookieName,
			Value:    token,
			Expires:  session.Expiration,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			Path:     "/",
		})

		// cleanup expired sessions
		if err := database.CleanSessions(db); err != nil {
			xhttp.Error(r.Context(), w, err)
			return
		}

		http.Redirect(w, r, redirectTo, http.StatusSeeOther)
	}))
}

// Cookie is a middleware that rejects requests without a valid cookie session.
// Also embeds the session in the request context for downstream handlers to use.
func (m *Manager) Cookie(db *wrap.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieName)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// get session / user
			var session database.Session
			err = db.View(func(txn *lmdb.Txn) error {
				dbis := db.GetDBis()
				sessDBI := dbis[database.SessionsDBIName]
				userDBI := dbis[database.UsersDBIName]
				// get session
				if err := database.TxnGetAndUnmarshal(txn, sessDBI, []byte(cookie.Value), &session); err != nil {
					return err
				}
				// get user
				return database.TxnGetAndUnmarshal(txn, userDBI, []byte(session.UserID.String()), &session.User)
			})
			if err != nil && lmdb.IsNotFound(err) {
				err = nil
			}

			// rate limit everything that's not a valid session
			if (session == (database.Session{})) || time.Now().After(session.Expiration) || (err != nil) {
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				if err := m.limit.Wait(ctx); err != nil { // could be err, timeout, or burst exceeded
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 429, Msg: "too many requests, try again later", Err: err})
					return
				}
			}

			// handle err response
			if err != nil {
				xhttp.Error(r.Context(), w, err)
				return
			}

			// handle missing / expired session
			if (session == (database.Session{})) || time.Now().After(session.Expiration) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// put session into context
			ctx := ContextWithSession(r.Context(), session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// genRandomString generates a cryptographically secure random token of n bytes that's URL and filename safe.
func genRandomString(size int) (string, error) {
	b := make([]byte, size)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
