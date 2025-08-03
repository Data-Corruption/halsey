package config

import (
	"fmt"
	"halsey/go/storage/database"

	"github.com/Data-Corruption/lmdb-go/lmdb"
)

type MigrationFunc func(txn *lmdb.Txn, dbi lmdb.DBI, schemas map[string]schema) error

var Migrations = map[string]MigrationFunc{
	"v1.0.0->v1.0.1": migrateV1_0_0toV1_0_1,
	"v0.0.1->v0.0.2": migrateV0_0_1toV0_0_2, // Example
}

func migrateV1_0_0toV1_0_1(txn *lmdb.Txn, dbi lmdb.DBI, schemas map[string]schema) error {
	// small update to add updateFollowup field. Write it's default value to the db
	if err := database.MarshalAndPut(txn, dbi, []byte("updateFollowup"), ""); err != nil {
		return fmt.Errorf("failed to marshal updateFollowup for %s: %w", schemas["version"], err)
	}
	return nil
}

// Example migration function
func migrateV0_0_1toV0_0_2(txn *lmdb.Txn, dbi lmdb.DBI, schemas map[string]schema) error {
	// Implement the migration logic here
	// Use old schema to read data and new schema to write updated data
	return nil
}
