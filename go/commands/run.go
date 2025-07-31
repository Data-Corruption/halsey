package commands

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/disc/listeners"
	"halsey/go/storage/assets"
	"halsey/go/storage/config"
	"net/http"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/urfave/cli/v3"
)

var Run = &cli.Command{
	Name:  "run",
	Usage: "runs the bot",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "register-commands",
			Aliases: []string{"rc"},
			Usage:   "register command definitions",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return run(ctx, cmd)
	},
}

func run(ctx context.Context, cmd *cli.Command) error {
	// router
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World\n"))
	})
	mux.HandleFunc("/a/{hash}", assets.AssetFS(ctx))

	// get http server related stuff from config
	port, err := config.Get[int](ctx, "port")
	if err != nil {
		return fmt.Errorf("failed to get port from config: %w", err)
	}
	useTLS, err := config.Get[bool](ctx, "useTLS")
	if err != nil {
		return fmt.Errorf("failed to get useTLS from config: %w", err)
	}
	tlsKeyPath, err := config.Get[string](ctx, "tlsKeyPath")
	if err != nil {
		return fmt.Errorf("failed to get tlsKeyPath from config: %w", err)
	}
	tlsCertPath, err := config.Get[string](ctx, "tlsCertPath")
	if err != nil {
		return fmt.Errorf("failed to get tlsCertPath from config: %w", err)
	}

	// create http server
	var srv *xhttp.Server
	srv, err = xhttp.NewServer(&xhttp.ServerConfig{
		Addr:        fmt.Sprintf(":%d", port),
		UseTLS:      useTLS,
		TLSKeyPath:  tlsKeyPath,
		TLSCertPath: tlsCertPath,
		Handler:     mux,
		AfterListen: func() {
			fmt.Printf("Server is listening on http://localhost%s\n", srv.Addr())
		},
		OnShutdown: func() {
			fmt.Println("shutting down, cleaning up resources ...")
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// put http server in ctx so bot can use it to exit gracefully
	ctx = context.WithValue(ctx, "httpServer", srv)

	// start bot
	if err = startBot(ctx, cmd.Bool("register-commands")); err != nil {
		return fmt.Errorf("failed to start bot: %w", err)
	}
	defer disc.Client.Close(context.TODO())
	if err := disc.Client.OpenGateway(context.TODO()); err != nil {
		return fmt.Errorf("failed to open gateway: %w", err)
	}

	// start http server
	if err := srv.Listen(); err != nil {
		return fmt.Errorf("server stopped with error: %w", err)
	} else {
		fmt.Println("server stopped gracefully")
	}

	return nil
}

// startBot initializes and starts the Discord bot client.
func startBot(ctx context.Context, registerCommands bool) error {
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
