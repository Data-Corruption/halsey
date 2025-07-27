package client

import (
	"context"

	"github.com/disgoorg/disgo/bot"
)

type contextKey struct{}

func IntoContext(ctx context.Context, client *bot.Client) context.Context {
	return context.WithValue(ctx, contextKey{}, client)
}

func FromContext(ctx context.Context) *bot.Client {
	if client, ok := ctx.Value(contextKey{}).(*bot.Client); ok {
		return client
	}
	return nil
}
