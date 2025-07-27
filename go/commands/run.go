package commands

import (
	"context"
	"fmt"
	"halsey/go/storage/config"
	"net/http"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/urfave/cli/v3"
)

var Run = &cli.Command{
	Name:  "run",
	Usage: "runs the bot",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return run(ctx)
	},
}

func run(ctx context.Context) error {
	// router
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World\n"))
	})

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

	// http server
	var srv *xhttp.Server
	srv, err = xhttp.NewServer(&xhttp.ServerConfig{
		Addr:        fmt.Sprintf(":%d", port),
		UseTLS:      useTLS,
		TLSKeyPath:  tlsKeyPath,
		TLSCertPath: tlsCertPath,
		Handler:     mux,
		OnShutdown: func() {
			fmt.Println("shutting down, cleaning up resources ...")
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// start bot (pass http server so it can be used in exit listener)

	// start http server
	if err := srv.Listen(); err != nil {
		return fmt.Errorf("server stopped with error: %w", err)
	} else {
		fmt.Println("server stopped gracefully")
	}

	return nil
}
