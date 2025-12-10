package database

import (
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

type RestartContext struct {
	RegisterCmds bool `json:"registerCmds"` // on startup, should we register commands
	// on startup if these are set, a followup message is sent to this interaction
	WasUpdate bool         `json:"wasUpdate"`
	IToken    string       `json:"interactionToken"`
	MessageID snowflake.ID `json:"messageID"`
}

// DomainBools represents per-domain toggling for a feature
// Instagram is not included cause they don't support small project / casual API usage.
// Twitter is not included cause the api pricing is stupid. Fuck you Elon, pathetic evil moron.
type DomainBools struct {
	Reddit        bool `json:"reddit"`
	RedGifs       bool `json:"redGifs"` // ;p
	YouTube       bool `json:"youTube"`
	YouTubeShorts bool `json:"youTubeShorts"`
}

type Configuration struct {
	LogLevel  string `json:"logLevel"`
	Port      int    `json:"port"`      // port the server is listening on. 80/443 will be omitted from URLs
	Host      string `json:"host"`      // host the server is listening on
	ProxyPort int    `json:"proxyPort"` // port the proxy is listening on, 0 = no proxy. 80/443 will be omitted from URLs

	UpdateNotifications bool      `json:"updateNotifications"`
	LastUpdateCheck     time.Time `json:"lastUpdateCheck"`
	UpdateAvailable     bool      `json:"updateAvailable"`

	// version when /update is accepted. This is lazily used to determine if the update was successful after restart.
	UpdateFollowup string `json:"updateFollowup"`
	ListenCounter  int    `json:"listenCounter"` // increment this on each service listen, used for detecting restarts

	RestartCtx RestartContext `json:"restartContext"`

	BotToken     string       `json:"botToken"`
	BotChannelID snowflake.ID `json:"botChannelID"`
	BioImageURL  string       `json:"bioImageURL"` // URL to an image for /about
}

type User struct {
	IsAdmin      bool        `json:"isAdmin"`
	Username     string      `json:"username"`
	AvatarURL    *string     `json:"avatarURL"`    // *string marshals as {null | "" | "x"}
	BackupOptOut bool        `json:"backupOptOut"` // skips backing up messages from this user
	AutoExpand   DomainBools `json:"autoExpand"`
}

type ChannelBackup struct {
	Enabled bool         `json:"backupEnabled"` // overruled if this is the bot channel
	Ceil    snowflake.ID `json:"backupCeil"`
	Head    snowflake.ID `json:"backupHead"`
	Tail    snowflake.ID `json:"backupTail"`
}

type Channel struct {
	GuildID snowflake.ID        `json:"guildID"`
	Name    string              `json:"name"`
	Type    discord.ChannelType `json:"type"`
	Backup  ChannelBackup       `json:"backup"`
	Deleted bool                `json:"deleted"` // for knowing to skip backup
}

type GuildBackup struct {
	Enabled bool   `json:"enabled"`
	RunID   string `json:"runID"` // for knowing if a backup is in progress, synchronizing the channels debugging, etc.
}

type Guild struct {
	Name         string              `json:"name"`
	FavChannelID snowflake.ID        `json:"favoriteChannelID"`
	SynctubeURL  string              `json:"synctubeURL"`
	PremiumTier  discord.PremiumTier `json:"premiumTier"`
	AntiRot      bool                `json:"antiRot"`
	Backup       GuildBackup         `json:"backup"`
}

type Session struct {
	UserID     snowflake.ID `json:"userID"`
	User       User         `json:"user"` // refreshed on each request
	Expiration time.Time    `json:"expiration"`
}
