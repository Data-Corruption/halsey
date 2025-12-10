package commands

import (
	"context"
	"fmt"
	"sprout/internal/app"
	"sprout/internal/discord/listeners"
	"sprout/internal/platform/database"
	"sprout/internal/platform/http/server"
	"sprout/internal/platform/http/server/router"
	"time"

	"github.com/Data-Corruption/stdx/xnet"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/urfave/cli/v3"
)

const (
	botShutdownTimeout = 10 * time.Second
)

var Service = register(func(a *app.App) *cli.Command {
	if !a.ServiceEnabled {
		return nil
	}
	return &cli.Command{
		Name:  "service",
		Usage: "service management commands",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// get service name / env file path
			if a.Name == "" || a.StorageDir == "" {
				return fmt.Errorf("app name or storage path not found")
			}
			serviceName := a.Name + ".service"
			envFilePath := fmt.Sprintf("%s/%s.env", a.StorageDir, a.Name)

			// print service management commands
			fmt.Printf("🖧 Service Cheat Sheet\n\n")
			fmt.Printf("    Status:  systemctl --user status %s\n", serviceName)
			fmt.Printf("    Enable:  systemctl --user enable %s\n", serviceName)
			fmt.Printf("    Disable: systemctl --user disable %s\n\n", serviceName)
			fmt.Printf("    Start:   systemctl --user start %s\n", serviceName)
			fmt.Printf("    Stop:    systemctl --user stop %s\n", serviceName)
			fmt.Printf("    Restart: systemctl --user restart %s\n\n", serviceName)
			fmt.Printf("    Reset:   systemctl --user reset-failed %s\n\n", serviceName)
			fmt.Printf("    Env:     edit %s then restart the service\n\n", envFilePath)
			fmt.Printf("    Logs:        journalctl --user -u %s -n 200 --no-pager\n", serviceName)
			fmt.Printf("    Update Logs: journalctl --user -u %s-update* -n 200 -f\n", a.Name)

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:        "run",
				Description: "Runs service in foreground. Typically called by systemd. If you need to run it manually/unmanaged, use this command.",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "rc",
						Usage: "register commands on startup",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					// wait for network (systemd user mode Wants/After is unreliable)
					if err := xnet.Wait(ctx, 0); err != nil {
						return fmt.Errorf("failed to wait for network: %w", err)
					}

					// get config
					cfg, err := database.ViewConfig(a.DB)
					if err != nil {
						return fmt.Errorf("failed to get configuration from database: %w", err)
					}

					// get port, handle override
					port := cmd.Int("port")
					if port == 0 {
						port = cfg.Port
					}

					// create server
					mux := router.New(a)
					if err := server.New(a, port, mux); err != nil {
						return fmt.Errorf("failed to create server: %w", err)
					}

					// start bot if token is set
					if cfg.BotToken != "" {
						if err := createClient(a, cfg.BotToken, cmd.Bool("rc")); err != nil {
							return fmt.Errorf("failed to create bot client: %w", err)
						}
					} else {
						a.Log.Warn("bot token not set in config, skipping bot startup")
					}

					// if created the client without issue, start it
					if a.Client != nil {
						a.AddCleanup(func() error {
							ctx, cancel := context.WithTimeout(context.Background(), botShutdownTimeout)
							defer cancel()
							a.Client.Close(ctx)
							return nil
						})
						if err := a.Client.OpenGateway(context.TODO()); err != nil {
							return fmt.Errorf("failed to open gateway: %w", err)
						}
					}

					// start http server
					if err := a.Server.Listen(); err != nil { // blocks until server stops or shutdown signal received
						return fmt.Errorf("server stopped with error: %w", err)
					} else {
						fmt.Println("server stopped gracefully")
					}

					return nil
				},
			},
		},
	}
})

func createClient(a *app.App, token string, registerCommands bool) error {
	a.Log.Debugf("creating client, disgo version: %s", disgo.Version)
	var err error
	a.Client, err = disgo.New(token,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds|
					gateway.IntentGuildMessages|
					gateway.IntentGuildMembers|
					gateway.IntentMessageContent, // includes attachments, etc. i think
			),
		),
		bot.WithCacheConfigOpts(
			cache.WithCaches(cache.FlagsAll),
		),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnReady:                         func(event *events.Ready) { listeners.OnReady(a, event) },
			OnGuildsReady:                   func(event *events.GuildsReady) { listeners.OnGuildsReady(a, event, registerCommands) },
			OnGuildUpdate:                   func(event *events.GuildUpdate) { listeners.OnGuildUpdate(a, event) },
			OnGuildMemberJoin:               func(event *events.GuildMemberJoin) { listeners.OnGuildMemberJoin(a, event) },
			OnGuildMessageCreate:            func(event *events.GuildMessageCreate) { listeners.OnGuildMessageCreate(a, event) },
			OnGuildChannelCreate:            func(event *events.GuildChannelCreate) { listeners.OnGuildChannelCreate(a, event) },
			OnGuildChannelDelete:            func(event *events.GuildChannelDelete) { listeners.OnGuildChannelDelete(a, event) },
			OnApplicationCommandInteraction: func(event *events.ApplicationCommandInteractionCreate) { listeners.OnCommandInteraction(a, event) },
			OnComponentInteraction:          func(event *events.ComponentInteractionCreate) { listeners.OnComponentInteraction(a, event) },
		}),
	)
	return err
}
