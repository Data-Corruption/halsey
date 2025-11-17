package commands

import (
	"context"
	"fmt"
	"net/http"
	"sprout/go/app"
	"sprout/go/discord/client"
	"sprout/go/discord/listeners"
	"sprout/go/platform/database/config"
	"sprout/go/platform/http/server"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/Data-Corruption/stdx/xnet"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/urfave/cli/v3"
)

func Service(a *app.App) *cli.Command {
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
				Action: func(ctx context.Context, cmd *cli.Command) error {
					// wait for network (systemd user mode Wants/After is unreliable)
					if err := xnet.Wait(ctx, 0); err != nil {
						return fmt.Errorf("failed to wait for network: %w", err)
					}

					var srv *xhttp.Server

					// create bot client
					bClient, err := disgo.New(token,
						bot.WithGatewayConfigOpts(
							gateway.WithIntents(gateway.IntentGuildScheduledEvents|gateway.IntentGuilds|gateway.IntentGuildMessages),
						),
						bot.WithCacheConfigOpts(
							cache.WithCaches(cache.FlagsAll),
						),
						bot.WithEventListeners(&events.ListenerAdapter{
							OnReady:                         func(event *events.Ready) { onReady(ctx, event) },
							OnGuildsReady:                   func(event *events.GuildsReady) { listeners.OnGuildsReady(ctx, event) },
							OnGuildMessageCreate:            func(event *events.GuildMessageCreate) { listeners.OnGuildMessageCreate(ctx, event) },
							OnApplicationCommandInteraction: func(event *events.ApplicationCommandInteractionCreate) { listeners.OnCommandInteraction(ctx, event) },
							OnComponentInteraction:          func(event *events.ComponentInteractionCreate) { listeners.OnComponentInteraction(ctx, event) },
						}),
					)
					if err != nil {
						return fmt.Errorf("failed to create bot client: %w", err)
					}
					ctx = client.IntoContext(ctx, bClient)

					// hello world handler
					mux := http.NewServeMux()
					mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
						w.Write([]byte("Hello World 4\n"))
					})
					mux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
						// daemon update example. add auth ofc, etc
						w.Write([]byte("Starting update...\n"))
						if err := update.Update(ctx, true); err != nil {
							xlog.Errorf(ctx, "/update update start failed: %s", err)
						}
						srv.Shutdown(nil)
					})

					// create server
					srv, err = server.New(ctx, mux)
					if err != nil {
						return fmt.Errorf("failed to create server: %w", err)
					}
					server.IntoContext(ctx, srv)

					// get general settings
					settings, err := config.Get[config.GeneralSettings](ctx, "generalSettings")
					if err != nil {
						return fmt.Errorf("failed to get general settings from config: %w", err)
					}

					// start bot if token is set
					if settings.BotToken != "" {
						if err = startBot(ctx); err != nil {
							return fmt.Errorf("failed to start bot: %w", err)
						}
						defer disc.Client.Close(context.TODO())
						if err := disc.Client.OpenGateway(context.TODO()); err != nil {
							return fmt.Errorf("failed to open gateway: %w", err)
						}
					} else {
						xlog.Warn(ctx, "bot token not set in config, skipping bot startup")
					}

					// start http server
					if err := srv.Listen(); err != nil {
						return fmt.Errorf("server stopped with error: %w", err)
					} else {
						fmt.Println("server stopped gracefully")
					}

					return nil
				},
			},
		},
	}
}

// startBot initializes and starts the Discord bot client.
func startBot(ctx context.Context) error {
	xlog.Info(ctx, "Starting Halsey...")
	xlog.Debug(ctx, fmt.Sprintf("disgo version: %s", disgo.Version))

	// get bot token from config
	token, err := config.Get[string](ctx, "botToken")
	if err != nil {
		return fmt.Errorf("bot token not set")
	}
	xlog.Debugf(ctx, "Using bot token: %s", token)
	if token == "" {
		return fmt.Errorf("bot token is empty")
	}

	// create bot client
	if disc.Client, err = disgo.New(token,
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(gateway.IntentGuildScheduledEvents|gateway.IntentGuilds|gateway.IntentGuildMessages),
		),
		bot.WithCacheConfigOpts(
			cache.WithCaches(cache.FlagsAll),
		),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnReady:                         func(event *events.Ready) { listeners.OnReady(ctx, event) },
			OnGuildsReady:                   func(event *events.GuildsReady) { listeners.OnGuildsReady(ctx, registerCommands, event) },
			OnGuildMessageCreate:            func(event *events.GuildMessageCreate) { listeners.OnGuildMessageCreate(ctx, event) },
			OnApplicationCommandInteraction: func(event *events.ApplicationCommandInteractionCreate) { listeners.OnCommandInteraction(ctx, event) },
			OnComponentInteraction:          func(event *events.ComponentInteractionCreate) { listeners.OnComponentInteraction(ctx, event) },
		}),
	); err != nil {
		return fmt.Errorf("failed to create bot client: %w", err)
	}

	return nil
}

func onReady(ctx context.Context, event *events.Ready) {
	fmt.Println("Halsey is now running. Press Ctrl+C to exit.")
	xlog.Info(ctx, "Halsey is now running.")
}
