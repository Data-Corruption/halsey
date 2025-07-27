package commands

import (
	"context"
	"fmt"
	"halsey/go/storage/config"

	"github.com/urfave/cli/v3"
)

var Config = &cli.Command{
	Name:  "config",
	Usage: "Set various configuration options",
	Commands: []*cli.Command{
		{
			Name:  "tls",
			Usage: "Set TLS configuration options",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "tlsKeyPath",
					Usage:    "Path to the TLS key file",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "tlsCertPath",
					Usage:    "Path to the TLS certificate file",
					Required: true,
				},
				&cli.BoolFlag{
					Name:  "useTLS",
					Usage: "Enable or disable TLS",
				},
			},
			Action: func(ctx context.Context, cmd *cli.Command) error {
				tlsKeyPath := cmd.String("tlsKeyPath")
				tlsCertPath := cmd.String("tlsCertPath")
				useTLS := cmd.Bool("useTLS")

				if err := config.Set(ctx, "tlsKeyPath", tlsKeyPath); err != nil {
					return fmt.Errorf("failed to set tlsKeyPath: %w", err)
				}
				if err := config.Set(ctx, "tlsCertPath", tlsCertPath); err != nil {
					return fmt.Errorf("failed to set tlsCertPath: %w", err)
				}
				if err := config.Set(ctx, "useTLS", useTLS); err != nil {
					return fmt.Errorf("failed to set useTLS: %w", err)
				}

				fmt.Println("TLS configuration updated successfully.")
				return nil
			},
		},
	},
}
