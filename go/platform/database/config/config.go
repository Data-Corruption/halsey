// Package config provides a typed, versioned configuration system with migration support. Here's how to use it and make changes:
//
// Basic Usage:
//
//	// Get a config value (type-safe)
//	port, err := config.Get[int](ctx, "port")
//
//	// Set a config value (type-safe)
//	err := config.Set[int](ctx, "port", 9000)
//
//	// See [Migrate]in `config.go` for a txn example
//
// Modifying the Schema:
//
//  1. Copy the current schema to a new version in SchemaRecord in `schema.go`.
//  2. Update the new schema with your changes.
//  3. Add migration functions in `migration.go` to handle the transition from the old schemas to the new one.
//
// see `migration.go` for example / details. This config impl may seem strange, this is due to me wanting a no compromise system that:
//
//   - is kinda type-safe
//   - has versioned schemas / can update between releases / migrate
//   - has transaction support / atomic updates across multiple process instances
package config

import (
	"encoding/json"
	"fmt"
	"sprout/go/platform/database"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/lmdb-go/wrap"
)

type valueInterface interface {
	DefaultValue() any
	GetAny(string, *wrap.DB) (any, error)
	SetAny(string, *wrap.DB, any) error
}

type value[T any] struct {
	d T // default value
}

func (v *value[T]) DefaultValue() any { return v.d }

func (v *value[T]) GetAny(key string, db *wrap.DB) (any, error) {
	data, err := db.Read(database.ConfigDBIName, []byte(key))
	if err != nil {
		return nil, fmt.Errorf("failed to read config key '%s': %w", key, err)
	}
	// Safeguard against unexpected empty data from storage (e.g., corruption, non-JSON write).
	// Treat as unset and return the default value; json.Marshal doesn't produce empty []byte for standard types.
	if len(data) == 0 {
		return nil, fmt.Errorf("config key '%s' has unexpected empty value in storage", key)
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal error for key '%s': %w", key, err)
	}
	return result, nil
}

func (v *value[T]) SetAny(key string, db *wrap.DB, val any) error {
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("marshal error for key '%s': %w", key, err)
	}
	return db.Write(database.ConfigDBIName, []byte(key), data) // update wrapper pkg to allow direct dbi use
}

type Config struct {
	Version    string
	Schemas    map[string]schema
	Migrations map[string]MigrationFunc // Key: "fromVersion->toVersion"
	DB         *wrap.DB
	DBI        lmdb.DBI // cached DBI for config
}

func New(version string, schemas map[string]schema, migrations map[string]MigrationFunc, db *wrap.DB) (*Config, error) { // separate from init for testing
	dbi, ok := db.GetDBis()[database.ConfigDBIName]
	if !ok {
		return nil, fmt.Errorf("config DBI not found in DB")
	}
	return &Config{
		Version:    version,
		Schemas:    schemas,
		Migrations: migrations,
		DB:         db,
		DBI:        dbi,
	}, nil
}

func Init(db *wrap.DB) (*Config, error) {
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	config, err := New(Version, SchemaRecord, Migrations, db)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}
	if err := config.Migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate config: %w", err)
	}
	return config, nil
}

func Get[T any](c *Config, key string) (T, error) {
	// Ensure the schema exists for the current version.
	cfgValue, exists := c.Schemas[c.Version][key]
	if !exists {
		return *new(T), fmt.Errorf("key %s not found in config", key)
	}
	// Assert that the schema definition is of the expected type.
	typedValue, ok := cfgValue.(*value[T])
	if !ok {
		return *new(T), fmt.Errorf("type mismatch for key %s", key)
	}
	// Use the GetAny method to retrieve the value.
	rawValue, err := typedValue.GetAny(key, c.DB)
	if err != nil {
		return *new(T), fmt.Errorf("failed to get config key '%s': %w", key, err)
	}
	// Assert that the retrieved value is of the expected type.
	result, ok := rawValue.(T)
	if !ok {
		return *new(T), fmt.Errorf("stored value for key %s is not of expected type", key)
	}
	return result, nil
}

func Set[T any](c *Config, key string, val T) error {
	// Ensure the schema exists for the current version.
	schemaForVersion, ok := c.Schemas[c.Version]
	if !ok {
		return fmt.Errorf("schema for version %s not found", c.Version)
	}
	// Retrieve the schema definition for the key.
	schemaVal, exists := schemaForVersion[key]
	if !exists {
		return fmt.Errorf("key %s not found in config", key)
	}
	// Assert that the schema definition is of the expected type.
	typedValue, ok := schemaVal.(*value[T])
	if !ok {
		return fmt.Errorf("type mismatch for key %s", key)
	}
	// Use the SetAny method to set the value.
	if err := typedValue.SetAny(key, c.DB, val); err != nil {
		return fmt.Errorf("failed to set config key '%s': %w", key, err)
	}
	return nil
}

// Migrate migrates or initializes the configuration in the database.
func (cfg *Config) Migrate() error {
	return cfg.DB.Update(func(txn *lmdb.Txn) error {
		var discVersion string
		if err := database.GetAndUnmarshal(txn, cfg.DBI, []byte("version"), &discVersion); err != nil {
			if !lmdb.IsNotFound(err) {
				return fmt.Errorf("failed to get config version: %w", err)
			}
			// no version found, initialize config
			for key, value := range cfg.Schemas[cfg.Version] {
				defaultValue := value.DefaultValue()
				if err := database.MarshalAndPut(txn, cfg.DBI, []byte(key), defaultValue); err != nil {
					return fmt.Errorf("failed to write initial value for key '%s': %w", key, err)
				}
			}
			fmt.Printf("config initialized with version '%s'\n", cfg.Version)
			return nil
		}

		// check if version is the latest
		if discVersion == cfg.Version {
			return nil
		}

		// migrate config
		migratePath := discVersion + "->" + cfg.Version
		fmt.Printf("config migration: %s\n", migratePath)
		if migrationFunc, ok := cfg.Migrations[migratePath]; ok {
			if err := migrationFunc(txn, cfg.DBI, cfg.Schemas); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}
			if err := database.MarshalAndPut(txn, cfg.DBI, []byte("version"), cfg.Version); err != nil {
				return fmt.Errorf("failed to write new version '%s': %w", cfg.Version, err)
			}
			fmt.Printf("config migration successful: %s\n", migratePath)
			return nil
		}
		// migration function not found
		return fmt.Errorf("unsupported migration path: from '%s' to '%s'. No migration function registered for this transition", discVersion, cfg.Version)
	})
}

// Print prints the current configuration to stdout.
// This is useful for debugging and verifying the current configuration state.
func (cfg *Config) Print() error {
	return cfg.DB.View(func(txn *lmdb.Txn) error {
		fmt.Printf("Current Configuration (Version: %s):\n", cfg.Version)
		for key, value := range cfg.Schemas[cfg.Version] {
			// skip sensitive fields like this
			// if key == "authToken" {
			//   fmt.Printf("%s: [REDACTED]\n", key)
			//   continue
			// }
			data, err := value.GetAny(key, cfg.DB)
			if err != nil {
				return fmt.Errorf("failed to get config key '%s': %w", key, err)
			}
			fmt.Printf("%s: %v\n", key, data)
		}
		return nil
	})
}
