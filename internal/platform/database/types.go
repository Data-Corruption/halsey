package database

import (
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

type RestartContext struct {
	RegisterCmds     bool   `json:"registerCmds"`     // on startup, should we register commands
	PreUpdateVersion string `json:"preUpdateVersion"` // version before update attempt, used for determining if update was successful
	ListenCounter    int    `json:"listenCounter"`    // increment this on each service listen, used for detecting restarts

	// for discord triggered restart followups
	IToken    string       `json:"interactionToken"`
	MessageID snowflake.ID `json:"messageID"`
}

// DomainBools represents per-domain toggling for a feature
// Instagram is not included cause they don't support small project / casual API usage.
// Twitter is not included cause the api pricing is stupid. Fuck you Elon, pathetic evil moron.
type DomainBools struct {
	Reddit        bool `json:"reddit"`
	RedGifs       bool `json:"redGifs"` // ;p
	YouTube       bool `json:"youTube"` // ignored by auto expand
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

	RestartCtx RestartContext `json:"restartContext"`

	BotToken string `json:"botToken"`
}

type User struct {
	IsAdmin      bool        `json:"isAdmin"` // set by admin
	Username     string      `json:"username"`
	AvatarURL    *string     `json:"avatarURL"`    // *string marshals as {null | "" | "x"}
	BackupAccess bool        `json:"backupAccess"` // allows user to download backups of guilds they are in, set by admin
	BackupOptOut bool        `json:"backupOptOut"` // skips backing up messages from this user
	AiAccess     bool        `json:"aiAccess"`     // allows user to use AI features, set by admin
	AiChatOptOut bool        `json:"aiChatOptOut"` // excludes this user from AI chat features
	AutoExpand   DomainBools `json:"autoExpand"`
}

type ChannelBackup struct {
	Enabled bool         `json:"backupEnabled"` // overruled if this is the bot channel
	Ceil    snowflake.ID `json:"backupCeil"`
	Head    snowflake.ID `json:"backupHead"`
	Tail    snowflake.ID `json:"backupTail"`
}

type Channel struct {
	GuildID  snowflake.ID        `json:"guildID"`
	Name     string              `json:"name"`
	Type     discord.ChannelType `json:"type"`
	Position int                 `json:"position"` // position in the channel list, guildThreads are always 0
	ParentID snowflake.ID        `json:"parentID"` // category ID, 0 if none
	Backup   ChannelBackup       `json:"backup"`
	Deleted  bool                `json:"deleted"` // for knowing to skip backup
}

type GuildBackup struct {
	Enabled  bool      `json:"enabled"`
	Password string    `json:"password"` // after daily backups are completed, this is used to create an encrypted zip file
	RunID    string    `json:"runID"`    // for knowing if a backup is in progress, synchronizing the channels debugging, etc.
	LastRun  time.Time `json:"lastRun"`  // for UI
}

type Guild struct {
	Name           string              `json:"name"`
	Members        []snowflake.ID      `json:"members"` // updated on guildReady and during guildMemberAdd / guildMemberLeave
	BotChannelID   snowflake.ID        `json:"botChannelID"`
	FavChannelID   snowflake.ID        `json:"favoriteChannelID"`
	SynctubeURL    string              `json:"synctubeURL"`
	PremiumTier    discord.PremiumTier `json:"premiumTier"`
	Backup         GuildBackup         `json:"backup"`
	AntiRotEnabled bool                `json:"antiRotEnabled"`
	AiChatEnabled  bool                `json:"aiChatEnabled"`
}

type Session struct {
	UserID     snowflake.ID `json:"userID"`
	User       User         `json:"user"` // refreshed on each request
	Expiration time.Time    `json:"expiration"`
}
