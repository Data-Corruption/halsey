// package auth provides a simple token-based authentication middleware for HTTP servers.
// Sessions generation happens using discord slash commands, are short lived, and sent via private
// messages to users. This is more than secure enough for the use case.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/Data-Corruption/lmdb-go/wrap"
	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/disgoorg/snowflake/v2"
	"golang.org/x/time/rate"
)

const (
	TokenParam       = "a"
	DefaultTTL       = 10 * time.Minute
	DefaultRateLimit = 1 * time.Second
	DefaultRateBurst = 5
)

var (
	ErrUninitialized      = errors.New("auth manager not initialized")
	ErrTokenCollision     = errors.New("generated token already exists")
	ErrNoSessionInContext = errors.New("no session in context")
)

type Manager struct {
	sessions map[string]Session // token -> session
	ttl      time.Duration
	limit    *rate.Limiter
	mu       sync.Mutex
	init     bool
}

// New creates a new auth manager. Nil arguments use defaults.
func New(ttl *time.Duration, limiter *rate.Limiter) *Manager {
	m := &Manager{
		sessions: make(map[string]Session),
		ttl:      DefaultTTL,
		limit:    rate.NewLimiter(rate.Every(DefaultRateLimit), DefaultRateBurst),
		init:     true,
	}

	if ttl != nil {
		m.ttl = *ttl
	}
	if limiter != nil {
		m.limit = limiter
	}

	return m
}

// NewSession generates a new token for the given user and stores it with an expiration time.
// It errors if the generated token collides with an existing one.
// Also cleans up expired sessions.
// Example URL with token and other params: /download?a=abcd1234&h=hash
func (m *Manager) NewSession(userID snowflake.ID) (string, error) {
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
	if _, exists := m.sessions[token]; exists {
		return "", ErrTokenCollision
	}

	expiration := time.Now().Add(m.ttl)
	m.sessions[token] = Session{
		UserID:     userID,
		Expiration: expiration,
	}

	// clean up expired sessions
	now := time.Now()
	for t, s := range m.sessions {
		if s.Expiration.Before(now) {
			delete(m.sessions, t)
		}
	}

	return token, nil
}

// Middleware rejects requests without a valid token.
// Also embeds the session in the request context for downstream handlers to use.
func (m *Manager) Middleware(db *wrap.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get(TokenParam)
			if token == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// ensure initialized / get session
			m.mu.Lock()
			if !m.init {
				m.mu.Unlock()
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "internal server error", Err: ErrUninitialized})
				return
			}
			session, exists := m.sessions[token]
			m.mu.Unlock()

			// handle missing / expired token
			if !exists || time.Now().After(session.Expiration) {
				// rate limit
				ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
				defer cancel()
				if err := m.limit.Wait(ctx); err != nil {
					// could be err, timeout, or burst exceeded
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 429, Msg: "too many requests, try again later", Err: err})
					return
				}
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// check if admin
			isAdmin, err := IsUserAdminByID(db, session.UserID)
			if err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "internal server error", Err: err})
				return
			}
			session.IsAdmin = isAdmin

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
