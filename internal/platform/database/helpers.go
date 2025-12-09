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

// ViewConfig retrieves a copy of the current configuration from the database.
func ViewConfig(db *wrap.DB) (*Configuration, error) {
	data, err := db.Read(ConfigDBIName, []byte(ConfigDataKey))
	if err != nil {
		return nil, err
	}
	var cfg Configuration
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateConfig updates the configuration in the database using the provided update function.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func UpdateConfig(db *wrap.DB, updateFunc func(cfg *Configuration) error) error {
	return db.Update(func(txn *lmdb.Txn) error {
		dbi, ok := db.GetDBis()[ConfigDBIName]
		if !ok {
			return fmt.Errorf("DBI %q not found", ConfigDBIName)
		}

		var cfg Configuration
		if err := TxnGetAndUnmarshal(txn, dbi, []byte(ConfigDataKey), &cfg); err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		if err := updateFunc(&cfg); err != nil {
			return fmt.Errorf("update function failed: %w", err)
		}

		if err := TxnMarshalAndPut(txn, dbi, []byte(ConfigDataKey), cfg); err != nil {
			return fmt.Errorf("failed to update config: %w", err)
		}

		return nil
	})
}

// ViewUser retrieves a copy of the given user from the database.
func ViewUser(db *wrap.DB, userID snowflake.ID) (*User, error) {
	if userID == 0 {
		return nil, fmt.Errorf("invalid user ID")
	}
	data, err := db.Read(UsersDBIName, []byte(userID.String()))
	if err != nil {
		return nil, err
	}
	var user User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}
	return &user, nil
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

	created := false

	if err := db.Update(func(txn *lmdb.Txn) error {
		dbi, ok := db.GetDBis()[UsersDBIName]
		if !ok {
			return fmt.Errorf("DBI %q not found", UsersDBIName)
		}

		var user User
		err := TxnGetAndUnmarshal(txn, dbi, []byte(userID.String()), &user)
		if err != nil {
			if !lmdb.IsNotFound(err) {
				return fmt.Errorf("failed to get user: %w", err)
			}
			created = true
			// initialize default user settings
			user = User{}
			user.AutoExpand.Reddit = true
			user.AutoExpand.RedGifs = true
			user.AutoExpand.YouTube = true
			user.AutoExpand.YouTubeShorts = true
		}

		if err := updateFunc(&user); err != nil {
			return fmt.Errorf("update function failed: %w", err)
		}

		if err := TxnMarshalAndPut(txn, dbi, []byte(userID.String()), user); err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}

		return nil
	}); err != nil {
		return false, err
	}

	return created, nil
}

// CleanSessions removes expired sessions from the database.
//
// WARNING: Starts a transaction. Avoid nesting transactions (deadlock risk).
func CleanSessions(db *wrap.DB) error {
	return db.Update(func(txn *lmdb.Txn) error {
		dbi, ok := db.GetDBis()[SessionsDBIName]
		if !ok {
			return fmt.Errorf("DBI %q not found", SessionsDBIName)
		}

		cursor, err := txn.OpenCursor(dbi)
		if err != nil {
			return fmt.Errorf("failed to create cursor: %w", err)
		}
		defer cursor.Close()

		// iterate through all sessions
		for {
			_, v, err := cursor.Get(nil, nil, lmdb.Next)
			if lmdb.IsNotFound(err) {
				break // no more entries
			}
			if err != nil {
				return fmt.Errorf("failed to get next session: %w", err)
			}

			var session Session
			if err := json.Unmarshal(v, &session); err != nil {
				return fmt.Errorf("failed to unmarshal session: %w", err)
			}

			if session.Expiration.Before(time.Now()) {
				if err := cursor.Del(0); err != nil {
					return fmt.Errorf("failed to delete session: %w", err)
				}
			}
		}

		return nil
	})
}
