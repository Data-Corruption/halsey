package router

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sprout/internal/app"
	"sprout/internal/platform/auth"
	"sprout/internal/platform/database"
	"sprout/internal/platform/http/server/router/css"
	"sprout/internal/platform/http/server/router/images"
	"sprout/internal/platform/http/server/router/js"
	"strings"
	"time"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/disgoorg/snowflake/v2"
	"github.com/go-chi/chi/v5"
)

//go:embed templates/settings.html
var tmplFS embed.FS

var tmpl = template.Must(template.ParseFS(tmplFS, "templates/settings.html"))

// RestartBody is the body of POST /settings/restart requests.
type RestartBody struct {
	RegisterCommands bool `json:"register_commands"`
	Update           bool `json:"update"`
}

func settingsRoutes(a *app.App, r *chi.Mux) {
	r.Route("/settings", func(s chi.Router) {
		s.Use(a.AuthManager.Cookie(a.DB))

		s.Get("/", func(w http.ResponseWriter, r *http.Request) {
			cfg, err := database.ViewConfig(a.DB)
			if err != nil {
				xhttp.Error(r.Context(), w, err)
				return
			}

			// get session from context
			session, ok := auth.SessionFromContext(r.Context())
			if !ok {
				xhttp.Error(r.Context(), w, auth.ErrNoSessionInContext)
				return
			}

			// get bot avatar
			app, err := a.Client.Rest.GetCurrentApplication()
			if err != nil {
				xhttp.Error(r.Context(), w, err)
				return
			}
			ext := "png"
			if strings.HasPrefix(*app.Bot.Avatar, "a_") {
				ext = "gif"
			}
			avatarURL := fmt.Sprintf(
				"https://cdn.discordapp.com/avatars/%s/%s.%s?size=256",
				app.ID,
				*app.Bot.Avatar,
				ext,
			)

			// Fetch guilds with channels and users for admin users
			var guilds []database.GuildWithID
			var users []database.UserWithID
			if session.User.IsAdmin {
				guilds, err = database.ViewAllGuildsWithChannels(a.DB)
				if err != nil {
					xhttp.Error(r.Context(), w, err)
					return
				}
				users, err = database.ViewUsers(a.DB)
				if err != nil {
					xhttp.Error(r.Context(), w, err)
					return
				}
			}

			data := map[string]any{
				"CSS":             css.Path(),
				"JS":              js.Path(),
				"Background":      images.Next(),
				"Favicon":         template.URL(`data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text x='50%' y='.9em' font-size='90' text-anchor='middle'>🖤</text></svg>`),
				"Title":           "Settings - Halsey",
				"Version":         a.Version,
				"UpdateAvailable": cfg.UpdateAvailable && (a.Version != "vX.X.X"),
				"User":            session.User,
				"AvatarURL":       template.URL(avatarURL),
				// Admin config fields
				"LogLevel":  cfg.LogLevel,
				"Port":      cfg.Port,
				"Host":      cfg.Host,
				"ProxyPort": cfg.ProxyPort,
				"HWAccel":   a.Compressor.GetHWAccel().String(),
				// Guild management
				"Guilds": guilds,
				// User management
				"Users": users,
			}
			if err := tmpl.Execute(w, data); err != nil {
				xhttp.Error(r.Context(), w, err)
				return
			}
		})

		// normal permissioned settings routes

		// Update user settings (non-admin)
		s.Post("/user", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			session, ok := auth.SessionFromContext(r.Context())
			if !ok {
				xhttp.Error(r.Context(), w, auth.ErrNoSessionInContext)
				return
			}

			// Parse body - all fields are optional
			var body struct {
				BackupOptOut *bool `json:"backupOptOut"`
				AiChatOptOut *bool `json:"aiChatOptOut"`
				AutoExpand   *struct {
					Reddit        *bool `json:"reddit"`
					YouTubeShorts *bool `json:"youTubeShorts"`
					RedGifs       *bool `json:"redGifs"`
				} `json:"autoExpand"`
			}
			dec := json.NewDecoder(r.Body)
			if err := dec.Decode(&body); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: err})
				return
			}

			// Update only the fields that were provided
			if _, err := database.UpsertUser(a.DB, session.UserID, func(user *database.User) error {
				if body.BackupOptOut != nil {
					user.BackupOptOut = *body.BackupOptOut
				}
				if body.AiChatOptOut != nil {
					user.AiChatOptOut = *body.AiChatOptOut
				}
				if body.AutoExpand != nil {
					if body.AutoExpand.Reddit != nil {
						user.AutoExpand.Reddit = *body.AutoExpand.Reddit
					}
					if body.AutoExpand.YouTubeShorts != nil {
						user.AutoExpand.YouTubeShorts = *body.AutoExpand.YouTubeShorts
					}
					if body.AutoExpand.RedGifs != nil {
						user.AutoExpand.RedGifs = *body.AutoExpand.RedGifs
					}
				}
				return nil
			}); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update user", Err: err})
				return
			}

			w.WriteHeader(http.StatusOK)
		})

		// Get download links for backups of guilds the user is a member of
		s.Get("/backups", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			// get session from context
			session, ok := auth.SessionFromContext(r.Context())
			if !ok {
				xhttp.Error(r.Context(), w, auth.ErrNoSessionInContext)
				return
			}
			if !(session.User.IsAdmin || session.User.BackupAccess) {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 403, Msg: "forbidden"})
				return
			}

			// Response struct for each guild backup
			type GuildBackupInfo struct {
				GuildName    string `json:"guildName"`
				DownloadLink string `json:"downloadLink"`
				LastRun      string `json:"lastRun"` // ISO 8601 format or empty
			}

			var backups []GuildBackupInfo

			// Get all guilds from the database (with channels already, but we just need guild info)
			guilds, err := database.ViewGuilds(a.DB)
			if err != nil {
				xhttp.Error(r.Context(), w, err)
				return
			}

			// Iterate through all guilds from the database
			token := ""
			for guildID, guild := range guilds {
				// Check if user is a member of this guild by checking the Members slice
				isMember := false
				for _, memberID := range guild.Members {
					if memberID == session.UserID {
						isMember = true
						break
					}
				}
				if !isMember {
					continue
				}

				// Skip guilds where backup is not enabled
				if !guild.Backup.Enabled {
					continue
				}

				// Generate a param session token for this download, only need one for all guilds
				if token == "" {
					token, err = a.AuthManager.NewParamSession(a.DB, session.UserID)
					if err != nil {
						a.Log.Warnf("failed to create param session for backup: %v", err)
						continue
					}
				}

				// Build the download link
				downloadLink := fmt.Sprintf("/download/backup/%s?%s=%s", guildID.String(), auth.ParamName, token)

				// Format LastRun time in RFC3339 (ISO 8601) for JavaScript localization
				lastRunStr := ""
				if !guild.Backup.LastRun.IsZero() {
					lastRunStr = guild.Backup.LastRun.Format(time.RFC3339)
				}

				backups = append(backups, GuildBackupInfo{
					GuildName:    guild.Name,
					DownloadLink: downloadLink,
					LastRun:      lastRunStr,
				})
			}

			// Return JSON response
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(backups); err != nil {
				xhttp.Error(r.Context(), w, err)
			}
		})

		// admin only routes.
		adminSettingsRoutes(a, s)
	})
}

func adminSettingsRoutes(a *app.App, r chi.Router) {
	r.Group(func(admin chi.Router) {
		admin.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// get session from context
				session, ok := auth.SessionFromContext(r.Context())
				if !ok {
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "internal server error", Err: auth.ErrNoSessionInContext})
					return
				}
				if !session.User.IsAdmin {
					xhttp.Error(r.Context(), w, &xhttp.Err{Code: 403, Msg: "forbidden", Err: fmt.Errorf("user %d is not an admin", session.UserID)})
					return
				}
				next.ServeHTTP(w, r)
			})
		})

		admin.Post("/restart", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			// parse body
			var body RestartBody
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields() // surfaces unexpected input early
			if err := dec.Decode(&body); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: err})
				return
			}
			if dec.More() {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: fmt.Errorf("invalid JSON: trailing data")})
				return
			}

			// should we update?
			var doUpdate bool
			if body.Update && a.Version != "vX.X.X" {
				doUpdate = true
			}

			// update restart context in config
			if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
				cfg.RestartCtx.RegisterCmds = body.RegisterCommands
				cfg.RestartCtx.ListenCounter = 0
				return nil
			}); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update config", Err: err})
				return
			}

			w.WriteHeader(http.StatusAccepted)

			// do the restart
			if doUpdate {
				if err := a.DetachUpdate(); err != nil {
					a.Log.Errorf("failed to detach update: %v", err)
				}
			} else {
				go a.Server.Shutdown()
			}
		})

		// Update admin configuration
		admin.Post("/admin", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			// Parse body - all fields are optional
			var body struct {
				LogLevel     *string `json:"logLevel"`
				Host         *string `json:"host"`
				Port         *int    `json:"port"`
				ProxyPort    *int    `json:"proxyPort"`
				BotToken     *string `json:"botToken"`
				SystemPrompt *string `json:"systemPrompt"`
			}
			dec := json.NewDecoder(r.Body)
			if err := dec.Decode(&body); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: err})
				return
			}

			// Update only the fields that were provided
			if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
				if body.LogLevel != nil {
					cfg.LogLevel = *body.LogLevel
				}
				if body.Host != nil {
					cfg.Host = *body.Host
				}
				if body.Port != nil {
					cfg.Port = *body.Port
				}
				if body.ProxyPort != nil {
					cfg.ProxyPort = *body.ProxyPort
				}
				if body.BotToken != nil && *body.BotToken != "" {
					cfg.BotToken = *body.BotToken
				}
				return nil
			}); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update config", Err: err})
				return
			}

			w.WriteHeader(http.StatusOK)
		})

		admin.Get("/restart-status", func(w http.ResponseWriter, r *http.Request) {
			cfg, err := database.ViewConfig(a.DB)
			if err != nil {
				xhttp.Error(r.Context(), w, err)
				return
			}

			restarted := cfg.RestartCtx.ListenCounter > 0
			updated := cfg.RestartCtx.PreUpdateVersion != "" && cfg.RestartCtx.PreUpdateVersion != a.Version

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]bool{"restarted": restarted, "updated": updated}); err != nil {
				xhttp.Error(r.Context(), w, err)
			}
		})

		// Update guild settings
		admin.Post("/guild/{guildID}", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			guildIDStr := chi.URLParam(r, "guildID")
			guildID, err := snowflake.Parse(guildIDStr)
			if err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "invalid guild ID", Err: err})
				return
			}

			// Parse body - all fields are optional
			var body struct {
				SynctubeURL    *string       `json:"synctubeURL"`
				BackupPassword *string       `json:"backupPassword"`
				BackupEnabled  *bool         `json:"backupEnabled"`
				AntiRotEnabled *bool         `json:"antiRotEnabled"`
				AiChatEnabled  *bool         `json:"aiChatEnabled"`
				SystemPrompt   *string       `json:"systemPrompt"`
				BotChannelID   *snowflake.ID `json:"botChannelID"`
				FavChannelID   *snowflake.ID `json:"favChannelID"`
			}
			dec := json.NewDecoder(r.Body)
			if err := dec.Decode(&body); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: err})
				return
			}

			// Update only the fields that were provided
			if _, err := database.UpsertGuild(a.DB, guildID, func(guild *database.Guild) error {
				if body.SynctubeURL != nil {
					guild.SynctubeURL = *body.SynctubeURL
				}
				if body.BackupPassword != nil {
					guild.Backup.Password = *body.BackupPassword
				}
				if body.BackupEnabled != nil {
					guild.Backup.Enabled = *body.BackupEnabled
				}
				if body.AntiRotEnabled != nil {
					guild.AntiRotEnabled = *body.AntiRotEnabled
				}
				if body.AiChatEnabled != nil {
					guild.AiChatEnabled = *body.AiChatEnabled
				}
				if body.BotChannelID != nil {
					guild.BotChannelID = *body.BotChannelID
				}
				if body.FavChannelID != nil {
					guild.FavChannelID = *body.FavChannelID
				}
				return nil
			}); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update guild", Err: err})
				return
			}

			w.WriteHeader(http.StatusOK)
		})

		// Update channel settings
		admin.Post("/channel/{channelID}", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			channelIDStr := chi.URLParam(r, "channelID")
			channelID, err := snowflake.Parse(channelIDStr)
			if err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "invalid channel ID", Err: err})
				return
			}

			// Parse body - all fields are optional
			var body struct {
				BackupEnabled *bool `json:"backupEnabled"`
			}
			dec := json.NewDecoder(r.Body)
			if err := dec.Decode(&body); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: err})
				return
			}

			// Update only the fields that were provided
			if _, err := database.UpsertChannel(a.DB, channelID, func(channel *database.Channel) error {
				if body.BackupEnabled != nil {
					channel.Backup.Enabled = *body.BackupEnabled
				}
				return nil
			}); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update channel", Err: err})
				return
			}

			w.WriteHeader(http.StatusOK)
		})

		// Update user permissions
		admin.Post("/user/{userID}", func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			userIDStr := chi.URLParam(r, "userID")
			userID, err := snowflake.Parse(userIDStr)
			if err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "invalid user ID", Err: err})
				return
			}

			// Parse body - all fields are optional
			var body struct {
				IsAdmin      *bool `json:"isAdmin"`
				BackupAccess *bool `json:"backupAccess"`
				AiAccess     *bool `json:"aiAccess"`
			}
			dec := json.NewDecoder(r.Body)
			if err := dec.Decode(&body); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 400, Msg: "bad request", Err: err})
				return
			}

			// Update only the fields that were provided
			if _, err := database.UpsertUser(a.DB, userID, func(user *database.User) error {
				if body.IsAdmin != nil {
					user.IsAdmin = *body.IsAdmin
				}
				if body.BackupAccess != nil {
					user.BackupAccess = *body.BackupAccess
				}
				if body.AiAccess != nil {
					user.AiAccess = *body.AiAccess
				}
				return nil
			}); err != nil {
				xhttp.Error(r.Context(), w, &xhttp.Err{Code: 500, Msg: "failed to update user", Err: err})
				return
			}

			w.WriteHeader(http.StatusOK)
		})
	})
}
