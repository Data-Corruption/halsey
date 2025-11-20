package commands

import (
	"context"
	"fmt"
	"sprout/go/app"
	"sprout/go/discord/listeners"
	"sprout/go/platform/database/config"
	"sprout/go/platform/http/server"
	"sprout/go/platform/http/server/router"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/Data-Corruption/stdx/xnet"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/urfave/cli/v3"
)

var Service = register(func(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "service",
		Usage: "service management commands",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// get service name / env file path
			if a.Name == "" || a.Paths.Storage == "" {
				return fmt.Errorf("app name or storage path not found")
			}
			serviceName := a.Name + ".service"
			envFilePath := fmt.Sprintf("%s/%s.env", a.Paths.Storage, a.Name)

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

					// create server
					mux := router.New(a)
					if err := server.New(a, mux); err != nil {
						return fmt.Errorf("failed to create server: %w", err)
					}

					// get general settings
					settings, err := config.Get[config.GeneralSettings](a.Config, "generalSettings")
					if err != nil {
						return fmt.Errorf("failed to get general settings from config: %w", err)
					}

					// start bot if token is set
					if settings.BotToken != "" {
						if err := createClient(a, settings.BotToken, cmd.Bool("rc")); err != nil {
							return fmt.Errorf("failed to create bot client: %w", err)
						}
					} else {
						xlog.Warn(ctx, "bot token not set in config, skipping bot startup")
					}

					// if created the client without issue, start it
					if a.Client != nil {
						defer a.Client.Close(context.TODO())
						if err := a.Client.OpenGateway(context.TODO()); err != nil {
							return fmt.Errorf("failed to open gateway: %w", err)
						}
					}

					// start http server
					if err := a.Net.Server.Listen(); err != nil { // blocks until server stops or shutdown signal received
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

func createClient(a *app.App, token string, rcFlag bool) error {
	a.Log.Info("Starting Halsey...")
	a.Log.Debugf("disgo version: %s", disgo.Version)
	// create bot client
	var err error
	if a.Client, err = disgo.New(token,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(gateway.IntentGuildScheduledEvents|gateway.IntentGuilds|gateway.IntentGuildMessages),
		),
		bot.WithCacheConfigOpts(
			cache.WithCaches(cache.FlagsAll),
		),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnReady:                         func(event *events.Ready) { listeners.OnReady(a, event) },
			OnGuildsReady:                   func(event *events.GuildsReady) { listeners.OnGuildsReady(a, event, rcFlag) },
			OnGuildMessageCreate:            func(event *events.GuildMessageCreate) { listeners.OnGuildMessageCreate(a, event) },
			OnApplicationCommandInteraction: func(event *events.ApplicationCommandInteractionCreate) { listeners.OnCommandInteraction(a, event) },
			OnComponentInteraction:          func(event *events.ComponentInteractionCreate) { listeners.OnComponentInteraction(a, event) },
		}),
	); err != nil {
		return fmt.Errorf("failed to create bot client: %w", err)
	}
	return nil
}
