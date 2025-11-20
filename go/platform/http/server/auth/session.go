package auth

import (
	"context"
	"slices"
	"sprout/go/platform/database/config"
	"time"

	"github.com/disgoorg/snowflake/v2"
)

type Session struct {
	UserID     snowflake.ID
	Expiration time.Time
}

type sessCtxKey struct{}

func SessionFromContext(ctx context.Context) (Session, bool) {
	sess, ok := ctx.Value(sessCtxKey{}).(Session)
	return sess, ok
}

func ContextWithSession(ctx context.Context, sess Session) context.Context {
	return context.WithValue(ctx, sessCtxKey{}, sess)
}

func IsUserAdminByID(cfg *config.Config, userID snowflake.ID) (bool, error) {
	settings, err := config.Get[config.GeneralSettings](cfg, "general_settings")
	if err != nil {
		return false, err
	}
	found := slices.Contains(settings.AdminWhitelist, userID)
	return found, nil
}
