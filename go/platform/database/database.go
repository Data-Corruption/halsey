// Package database provides functions to manage the LMDB wrapper for the application.
package database

import (
	"encoding/json"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/disgoorg/snowflake/v2"
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
Users
	<id> -> marshaled User struct
Channels
	<id> -> marshaled Channel struct
Guilds
	<id> -> marshaled Guild struct

*/

const (
	ConfigDBIName    = "config"
	ArchiveDBIName   = "archive"
	AssetsDBIName    = "assets"
	FavoritesDBIName = "favorites"
	UsersDBIName     = "users"
	ChannelsDBIName  = "channels"
	GuildsDBIName    = "guilds"
	// Add more DBI names as needed, e.g., UserDBIName, SessionDBIName, etc. Also update the slice below to include them.
	// WARNING: If you add more DBIs you'll need to clean and reinitialize the database from scratch pretty sure.
)

// Slice for easy initialization. As stated above, if you add more DBIs you'll need to update this slice as well.
var DBINameList = []string{ConfigDBIName, ArchiveDBIName, AssetsDBIName, FavoritesDBIName, UsersDBIName, ChannelsDBIName, GuildsDBIName}

type User struct {
	Backups    bool `json:"backups"`
	AntiRot    bool `json:"antiRot"`
	AutoExpand struct {
		Instagram bool `json:"instagram"`
		Reddit    bool `json:"reddit"`
		Twitter   bool `json:"twitter"`
		YouTube   bool `json:"youTube"`
	} `json:"autoExpand"`
}

type Channel struct {
	GuildID snowflake.ID `json:"guildID"`
	Backup  struct {
		Enabled bool         `json:"backupEnabled"`
		Ceil    snowflake.ID `json:"backupCeil"`
		Head    snowflake.ID `json:"backupHead"`
		Tail    snowflake.ID `json:"backupTail"`
	} `json:"backup"`
}

type Guild struct {
	Name         string       `json:"name"`
	FavChannelID snowflake.ID `json:"favoriteChannelID"`
	SynctubeURL  string       `json:"synctubeURL"`
	PremiumTier  int          `json:"premiumTier"` // cached boost level
	Backup       struct {
		Enabled bool   `json:"enabled"`
		RunID   string `json:"runID"` // for knowing if a backup is in progress, debugging, etc.
	} `json:"backup"`
}

// ---- Helpers ---------------------------------------------------------------

/* Example get dbi from db
dbi, ok := db.GetDBis()[dbiName]
if !ok { ... }
*/

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
