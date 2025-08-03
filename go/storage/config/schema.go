package config

import "time"

/*
Once used in a released version, this struct cannot be changed.
If you need to change it, create a new struct and use it in the migration.

type ExampleV2 struct {
	x string
}

type Example struct {
	x int
}
*/

// Version is the current version of the schema
const Version = "v1.0.1"

// key -> default value
type schema map[string]valueInterface

// SchemaRecord is a version -> schema map of all released and the current schema. For defaults and migration purposes.
// After making changes to the schema, before the next release you must add a new version entry to this variable
// and migration funcs for it in `migration.go`. The newest version is assumed to be the current version.
var SchemaRecord = map[string]schema{
	"v1.0.1": {
		"version":           &value[string]{"v1.0.1"},
		"logLevel":          &value[string]{"warn"},
		"port":              &value[int]{8080},
		"hostname":          &value[string]{""}, // empty means no file hosting / serving
		"useTLS":            &value[bool]{false},
		"tlsKeyPath":        &value[string]{""},
		"tlsCertPath":       &value[string]{""},
		"updateNotify":      &value[bool]{true},
		"lastUpdateCheck":   &value[string]{time.Now().Format(time.RFC3339)},
		"updateAvailable":   &value[bool]{false},
		"botToken":          &value[string]{""},           // WARNING: if you change this, update the print func in config.go
		"backupPassword":    &value[string]{""},           // WARNING: if you change this, update the print func in config.go
		"uploadSizeLimitMB": &value[uint]{10},             // 10 MB default upload size limit
		"botChannelID":      &value[string]{""},           // internal channel for privileged updates, etc.
		"adminUserIDs":      &value[[]string]{[]string{}}, // list of user IDs that are considered admins
		"bioURL":            &value[string]{""},           // URL to the bio image, used in /about
		"biohURL":           &value[string]{""},           // URL to the bioh image, used in /send nudes
		"updateFollowup":    &value[string]{""},           // followup "<interaction token>|<message ID>" for update command response.
	},
	"v1.0.0": {
		"version":           &value[string]{"v1.0.0"},
		"logLevel":          &value[string]{"warn"},
		"port":              &value[int]{8080},
		"hostname":          &value[string]{""}, // empty means no file hosting / serving
		"useTLS":            &value[bool]{false},
		"tlsKeyPath":        &value[string]{""},
		"tlsCertPath":       &value[string]{""},
		"updateNotify":      &value[bool]{true},
		"lastUpdateCheck":   &value[string]{time.Now().Format(time.RFC3339)},
		"updateAvailable":   &value[bool]{false},
		"botToken":          &value[string]{""},           // WARNING: if you change this, update the print func in config.go
		"backupPassword":    &value[string]{""},           // WARNING: if you change this, update the print func in config.go
		"uploadSizeLimitMB": &value[uint]{10},             // 10 MB default upload size limit
		"botChannelID":      &value[string]{""},           // internal channel for privileged updates, etc.
		"adminUserIDs":      &value[[]string]{[]string{}}, // list of user IDs that are considered admins
		"bioURL":            &value[string]{""},           // URL to the bio image, used in /about
		"biohURL":           &value[string]{""},           // URL to the bioh image, used in /send nudes
	},
	/*
		"v0.0.2": {
			"version": &value[string]{"v0.0.2"},
			"example1": &value[bool]{true},
			"example3": &value[ExampleV2]{ExampleV2{"value"}},
		},
		"v0.0.1": {
			"version": &value[string]{"v0.0.1"},
			"example1": &value[string]{"value"},
			"example2": &value[int]{0},
			"example3": &value[Example]{Example{1}},
		},
	*/
}
