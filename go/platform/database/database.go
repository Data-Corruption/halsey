// Package database provides functions to manage the LMDB wrapper for the application.
package database

import (
	"context"
	"errors"
	"path/filepath"
	"sprout/go/app"

	"github.com/Data-Corruption/lmdb-go/wrap"
)

/*
Database Layout:

Config - see config package for details.

Archive
	<message id> -> gzipped message
Assets
	<url> -> <hash.ext> (<hash of hashes>.tar for galleries, etc)
Favorites
	<source message id> -> ID of copy in fav channel
Channels
	<id>.guild -> parent guild ID
	<id>.backup.exclude -> bool indicating if channel is excluded from backups.
	<id>.backup.ceil ->
	<id>.backup.head ->
	<id>.backup.tail ->
Guilds
	<id>.name ->
	<id>.backup.run -> backup run ID, used to synchronize / debug.
	<id>.backup.enabled ->
	<id>.favoriteChannelID ->
	<id>.boostLevel -> cached boost level
	<id>.synctubeURL -> url that lets us watch stuff together.

*/

const (
	ConfigDBIName = "config"
	// Add more DBI names as needed, e.g., UserDBIName, SessionDBIName, etc. Also update the slice below to include them.
	// WARNING: If you add more DBIs you'll need to clean and reinitialize the database from scratch pretty sure.
)

type ctxKey struct{}

func IntoContext(ctx context.Context, db *wrap.DB) context.Context {
	return context.WithValue(ctx, ctxKey{}, db)
}

func FromContext(ctx context.Context) *wrap.DB {
	if db, ok := ctx.Value(ctxKey{}).(*wrap.DB); ok {
		return db
	}
	return nil
}

func New(ctx context.Context) (*wrap.DB, error) {
	appInfo, ok := app.FromContext(ctx)
	if !ok {
		return nil, errors.New("app info not found in context")
	}
	if appInfo.Storage == "" {
		return nil, errors.New("nexus data path not set before database initialization")
	}
	db, _, err := wrap.New(filepath.Join(appInfo.Storage, "db"),
		[]string{ConfigDBIName}, // If you add more DBIs, update this slice as well.
	)
	if err != nil && db != nil {
		db.Close()
	}
	return db, err
}
