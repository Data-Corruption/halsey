// Package database provides functions to manage the LMDB wrapper for the application.
package database

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"

	"halsey/go/storage/storagepath"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/lmdb-go/wrap"
)

/*
Database Layout:

Config - see config package for details.

Backup
	<message id> -> gzipped message
Assets
	ref.<hash> -> []string message references.
	hash.<url> -> hash (for stuff not hosted by us that we backed up)
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
	<id>.uploadLimitMB -> message upload limit.
	<id>.synctubeURL -> url that lets us watch stuff together.

*/

const (
	ConfigDBIName    = "config"
	GuildsDBIName    = "guilds"
	ChannelsDBIName  = "channels"
	FavoritesDBIName = "favorites"
	AssetsDBIName    = "assets"
	BackupDBIName    = "backup"
	// Add more DBI names as needed, e.g., UserDBIName, SessionDBIName, etc.
	// WARNING: If you add more DBIs you'll need to clean and reinitialize the database from scratch pretty sure.
	// Also update the New function below to include them in the DBI slice.
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
	path := storagepath.FromContext(ctx)
	if path == "" {
		return nil, errors.New("nexus data path not set before database initialization")
	}
	db, _, err := wrap.New(filepath.Join(path, "db"),
		[]string{ConfigDBIName, GuildsDBIName, ChannelsDBIName, FavoritesDBIName, AssetsDBIName, BackupDBIName},
	)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// helper funcs

// basic txn ops

func MarshalAndPut(txn *lmdb.Txn, dbi lmdb.DBI, key []byte, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := txn.Put(dbi, key, data, 0); err != nil {
		return err
	}
	return nil
}

// lmdb.IsNotFound(err) will be true if the key was not found in the database.
func GetAndUnmarshal(txn *lmdb.Txn, dbi lmdb.DBI, key []byte, value any) error {
	buf, err := txn.Get(dbi, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(buf, value); err != nil {
		return err
	}
	return nil
}

// getting db stuff

func GetDbAndDBI(ctx context.Context, dbiName string) (*wrap.DB, lmdb.DBI, error) {
	db := FromContext(ctx)
	if db == nil {
		return nil, 0, errors.New("database not found in context")
	}
	dbi, ok := db.GetDBis()[dbiName]
	if !ok {
		return nil, 0, errors.New("DBI not found in database: " + dbiName)
	}
	return db, dbi, nil
}
