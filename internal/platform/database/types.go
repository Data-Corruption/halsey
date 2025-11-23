package database

import (
	"time"

	"github.com/disgoorg/snowflake/v2"
)

type RestartContext struct {
	RegisterCmds bool `json:"registerCmds"` // on startup, should we register commands
	// on startup if these are set, a followup message is sent to this interaction
	IToken    string       `json:"interactionToken"`
	MessageID snowflake.ID `json:"messageID"`
}

type DomainBools struct {
	Instagram bool `json:"instagram"`
	Reddit    bool `json:"reddit"`
	Twitter   bool `json:"twitter"`
	YouTube   bool `json:"youTube"`
}

type Configuration struct {
	LogLevel  string `json:"logLevel"`
	Port      int    `json:"port"`      // port the server is listening on. 80/443 will be omitted from URLs
	Host      string `json:"host"`      // host the server is listening on
	ProxyPort int    `json:"proxyPort"` // port the proxy is listening on, 0 = no proxy. 80/443 will be omitted from URLs

	UpdateNotifications bool      `json:"updateNotifications"`
	LastUpdateCheck     time.Time `json:"lastUpdateCheck"`
	UpdateAvailable     bool      `json:"updateAvailable"`

	RestartCtx RestartContext `json:"restartContext"`

	BotToken     string       `json:"botToken"`
	BotChannelID snowflake.ID `json:"botChannelID"`
	BioImageURL  string       `json:"bioImageURL"` // URL to an image for /about

	Downloads DomainBools `json:"downloads"` // global download toggles. For backup, expanding, etc.
}

type User struct {
	IsAdmin bool `json:"isAdmin"`
	Backups bool `json:"backups"`
	AntiRot bool `json:"antiRot"`
	// per-domain auto expand settings for this user
	AutoExpand DomainBools `json:"autoExpand"`
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
	AntiRot      bool         `json:"antiRot"`
	Backup       struct {
		Enabled bool   `json:"enabled"`
		RunID   string `json:"runID"` // for knowing if a backup is in progress, debugging, etc.
	} `json:"backup"`
}
