package auth

import (
	"context"
	"sprout/internal/platform/database"
)

type sessCtxKey struct{}

func SessionFromContext(ctx context.Context) (database.Session, bool) {
	sess, ok := ctx.Value(sessCtxKey{}).(database.Session)
	return sess, ok
}

func ContextWithSession(ctx context.Context, sess database.Session) context.Context {
	return context.WithValue(ctx, sessCtxKey{}, sess)
}
