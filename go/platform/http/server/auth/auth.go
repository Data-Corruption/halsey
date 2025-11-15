// package auth provides an ultra simple token-based authentication middleware for HTTP servers.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"sprout/go/platform/x"
	"sync"
	"time"

	"github.com/Data-Corruption/stdx/xhttp"
	"golang.org/x/time/rate"
)

const (
	TokenParam = "token"
	DefaultTTL = 10 * time.Minute
	// use case is very small server / user count, is tuned accordingly
	DefaultRateLimit = 1 * time.Second
	DefaultRateBurst = 5
)

var ErrUninitialized = errors.New("auth manager not initialized")

type Manager struct {
	tokens map[string]time.Time // token -> expiration time
	ttl    time.Duration
	limit  *rate.Limiter
	mu     sync.Mutex
	init   bool
}

// New creates a new auth manager. Nil arguments use defaults.
func New(ttl *time.Duration, limiter *rate.Limiter) *Manager {
	m := &Manager{tokens: make(map[string]time.Time), init: true}
	m.ttl = x.Ternary(ttl == nil, DefaultTTL, *ttl)
	m.limit = x.Ternary(limiter == nil, rate.NewLimiter(rate.Every(DefaultRateLimit), DefaultRateBurst), limiter)
	return m
}

// NewToken generates a new token and stores it with an expiration time.
// Also cleans up expired tokens.
func (m *Manager) NewToken() (string, error) {
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
	expiration := time.Now().Add(m.ttl)
	m.tokens[token] = expiration

	// clean up expired tokens
	now := time.Now()
	for t, exp := range m.tokens {
		if exp.Before(now) {
			delete(m.tokens, t)
		}
	}

	return token, nil
}

// Auth is middleware that rejects requests without a valid token.
// Example url with token: /download?token=abcd1234
func (m *Manager) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get(TokenParam)
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// ensure initialized / get token
		m.mu.Lock()
		if !m.init {
			m.mu.Unlock()
			xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "internal server error", Err: ErrUninitialized})
			return
		}
		expiration, exists := m.tokens[token]
		m.mu.Unlock()

		// handle missing / expired token
		if !exists || time.Now().After(expiration) {
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

		next.ServeHTTP(w, r)
	})
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
