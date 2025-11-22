package auth

import (
	"context"
	"sprout/go/platform/database"
	"time"

	"github.com/Data-Corruption/lmdb-go/wrap"
	"github.com/disgoorg/snowflake/v2"
)

type Session struct {
	UserID     snowflake.ID
	IsAdmin    bool
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

// IsUserAdminByID checks if a user is an admin by their user ID.
// Returns true if the user exists and is an admin, false otherwise.
// See lmdb.IsNotFound(err) for branching on user not found vs other errors.
func IsUserAdminByID(db *wrap.DB, userID snowflake.ID) (bool, error) {
	user, err := database.ViewUser(db, userID)
	if err != nil {
		return false, err
	}
	if user != nil && user.IsAdmin {
		return true, nil
	}
	return false, nil
}
