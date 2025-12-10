package database

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/lmdb-go/wrap"
	"github.com/disgoorg/snowflake/v2"
)

// TxnMarshalAndPut marshals the provided value and stores it in the database under the given key.
func TxnMarshalAndPut(txn *lmdb.Txn, dbi lmdb.DBI, key []byte, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := txn.Put(dbi, key, data, 0); err != nil {
		return err
	}
	return nil
}

// TxnGetAndUnmarshal retrieves a value from the database and unmarshals it into the provided value pointer.
// lmdb.IsNotFound(err) will be true if the key was not found in the database.
func TxnGetAndUnmarshal(txn *lmdb.Txn, dbi lmdb.DBI, key []byte, value any) error {
	buf, err := txn.Get(dbi, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(buf, value); err != nil {
		return err
	}
	return nil
}

// --- Generic Helpers ---

// View retrieves a copy of a value from the database.
// lmdb.IsNotFound(err) will be true if the key was not found.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func View[T any](db *wrap.DB, dbiName string, key []byte) (*T, error) {
	data, err := db.Read(dbiName, key)
	if err != nil {
		return nil, err
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return &value, nil
}

// Upsert updates a value in the database using the provided update function,
// creating it with defaultFn if it does not exist.
// Returns true if the value was created.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func Upsert[T any](db *wrap.DB, dbiName string, key []byte, defaultFn func() T, updateFn func(*T) error) (bool, error) {
	created := false

	if err := db.Update(func(txn *lmdb.Txn) error {
		dbi, ok := db.GetDBis()[dbiName]
		if !ok {
			return fmt.Errorf("DBI %q not found", dbiName)
		}

		var value T
		err := TxnGetAndUnmarshal(txn, dbi, key, &value)
		if err != nil {
			if !lmdb.IsNotFound(err) {
				return fmt.Errorf("failed to get value: %w", err)
			}
			created = true
			value = defaultFn()
		}

		if err := updateFn(&value); err != nil {
			return fmt.Errorf("update function failed: %w", err)
		}

		if err := TxnMarshalAndPut(txn, dbi, key, value); err != nil {
			return fmt.Errorf("failed to update value: %w", err)
		}

		return nil
	}); err != nil {
		return false, err
	}

	return created, nil
}

// ForEachAction specifies what to do with an entry after the callback.
type ForEachAction int

const (
	Keep   ForEachAction = iota // no changes to entry
	Update                      // re-marshal and store entry
	Delete                      // remove entry
)

// ForEach iterates over all entries in a DBI, applying the callback to each.
// The callback receives the key and a pointer to the unmarshaled value.
// Return (Keep, nil) to leave unchanged, (Update, nil) to save changes, (Delete, nil) to remove.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func ForEach[T any](db *wrap.DB, dbiName string, callback func(key []byte, value *T) (ForEachAction, error)) error {
	return db.Update(func(txn *lmdb.Txn) error {
		dbi, ok := db.GetDBis()[dbiName]
		if !ok {
			return fmt.Errorf("DBI %q not found", dbiName)
		}

		cursor, err := txn.OpenCursor(dbi)
		if err != nil {
			return fmt.Errorf("failed to create cursor: %w", err)
		}
		defer cursor.Close()

		for {
			k, v, err := cursor.Get(nil, nil, lmdb.Next)
			if lmdb.IsNotFound(err) {
				break // no more entries
			}
			if err != nil {
				return fmt.Errorf("failed to get next entry: %w", err)
			}

			var value T
			if err := json.Unmarshal(v, &value); err != nil {
				return fmt.Errorf("failed to unmarshal entry: %w", err)
			}

			action, err := callback(k, &value)
			if err != nil {
				return fmt.Errorf("callback failed: %w", err)
			}

			switch action {
			case Update:
				if err := TxnMarshalAndPut(txn, dbi, k, value); err != nil {
					return fmt.Errorf("failed to update entry: %w", err)
				}
			case Delete:
				if err := cursor.Del(0); err != nil {
					return fmt.Errorf("failed to delete entry: %w", err)
				}
			}
		}
		return nil
	})
}

// --- Type-Specific Wrappers ---

// ViewConfig retrieves a copy of the current configuration from the database.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func ViewConfig(db *wrap.DB) (*Configuration, error) {
	return View[Configuration](db, ConfigDBIName, []byte(ConfigDataKey))
}

func defaultConfig() Configuration {
	return Configuration{
		LogLevel:            "WARN",
		Port:                8080,
		Host:                "localhost",
		UpdateNotifications: true,
		LastUpdateCheck:     time.Now(),
	}
}

// UpdateConfig updates the configuration in the database using the provided update function.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func UpdateConfig(db *wrap.DB, updateFunc func(cfg *Configuration) error) error {
	_, err := Upsert(db, ConfigDBIName, []byte(ConfigDataKey), defaultConfig, updateFunc)
	return err
}

// ViewUser retrieves a copy of the given user from the database.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func ViewUser(db *wrap.DB, userID snowflake.ID) (*User, error) {
	if userID == 0 {
		return nil, fmt.Errorf("invalid user ID")
	}
	return View[User](db, UsersDBIName, []byte(userID.String()))
}

// defaultUser returns a User with default settings.
func defaultUser() User {
	return User{
		AutoExpand: DomainBools{
			Reddit:        true,
			RedGifs:       true,
			YouTube:       true,
			YouTubeShorts: true,
		},
	}
}

// UpsertUser updates the given user in the database using the provided
// update function, creating the user if they do not already exist.
// It returns a boolean indicating whether the user was created.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func UpsertUser(db *wrap.DB, userID snowflake.ID, updateFunc func(user *User) error) (bool, error) {
	if userID == 0 {
		return false, fmt.Errorf("invalid user ID")
	}
	return Upsert(db, UsersDBIName, []byte(userID.String()), defaultUser, updateFunc)
}

// ViewChannel retrieves a copy of the given channel from the database.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func ViewChannel(db *wrap.DB, channelID snowflake.ID) (*Channel, error) {
	if channelID == 0 {
		return nil, fmt.Errorf("invalid channel ID")
	}
	return View[Channel](db, ChannelsDBIName, []byte(channelID.String()))
}

func defaultChannel() Channel {
	return Channel{
		Backup: ChannelBackup{
			Enabled: true,
		},
	}
}

// UpsertChannel updates the given channel in the database using the provided
// update function, creating the channel if it does not already exist.
// It returns a boolean indicating whether the channel was created.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func UpsertChannel(db *wrap.DB, channelID snowflake.ID, updateFunc func(channel *Channel) error) (bool, error) {
	if channelID == 0 {
		return false, fmt.Errorf("invalid channel ID")
	}
	return Upsert(db, ChannelsDBIName, []byte(channelID.String()), defaultChannel, updateFunc)
}

// UpdateChannels updates all the channels in the database with the provided update function.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func UpdateChannels(db *wrap.DB, updateFunc func(id snowflake.ID, channel *Channel) error) error {
	return ForEach(db, ChannelsDBIName, func(key []byte, channel *Channel) (ForEachAction, error) {
		id, err := snowflake.Parse(string(key))
		if err != nil {
			return Keep, fmt.Errorf("failed to parse channel ID: %w", err)
		}
		if err := updateFunc(id, channel); err != nil {
			return Keep, err
		}
		return Update, nil
	})
}

// ViewGuild retrieves a copy of the given guild from the database.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func ViewGuild(db *wrap.DB, guildID snowflake.ID) (*Guild, error) {
	if guildID == 0 {
		return nil, fmt.Errorf("invalid guild ID")
	}
	return View[Guild](db, GuildsDBIName, []byte(guildID.String()))
}

func defaultGuild() Guild {
	return Guild{}
}

// UpsertGuild updates the given guild in the database using the provided
// update function, creating the guild if it does not already exist.
// It returns a boolean indicating whether the guild was created.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func UpsertGuild(db *wrap.DB, guildID snowflake.ID, updateFunc func(guild *Guild) error) (bool, error) {
	if guildID == 0 {
		return false, fmt.Errorf("invalid guild ID")
	}
	return Upsert(db, GuildsDBIName, []byte(guildID.String()), defaultGuild, updateFunc)
}

// CleanSessions removes expired sessions from the database.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func CleanSessions(db *wrap.DB) error {
	return ForEach(db, SessionsDBIName, func(_ []byte, session *Session) (ForEachAction, error) {
		if session.Expiration.Before(time.Now()) {
			return Delete, nil
		}
		return Keep, nil
	})
}
