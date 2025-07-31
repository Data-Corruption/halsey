package commands

import (
	"context"
	"fmt"
	"halsey/go/storage/config"
	"strconv"

	"github.com/urfave/cli/v3"
)

var Config = &cli.Command{
	Name:  "config",
	Usage: "Set various configuration options",
	Commands: []*cli.Command{
		{
			Name:  "print",
			Usage: "Print the current configuration",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				cfg := config.FromContext(ctx)
				if cfg == nil {
					return fmt.Errorf("no configuration found in context")
				}
				return cfg.Print()
			},
		},
		{
			Name:  "port",
			Usage: "Set the server port `<port>`",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				port := cmd.Args().Get(0)
				if port == "" {
					return fmt.Errorf("port cannot be empty")
				}
				// convert port to int
				portInt, err := strconv.Atoi(port)
				if err != nil {
					return fmt.Errorf("invalid port: %w", err)
				}
				// set port in config
				if err := config.Set(ctx, "port", portInt); err != nil {
					return fmt.Errorf("failed to set port: %w", err)
				}
				fmt.Printf("Port set to %d\n", portInt)
				return nil
			},
		},
		{
			Name:  "hostname",
			Usage: "Set the server hostname `<hostname>`",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				hostname := cmd.Args().Get(0)
				if hostname == "" {
					return fmt.Errorf("hostname cannot be empty")
				}
				// set hostname in config
				if err := config.Set(ctx, "hostname", hostname); err != nil {
					return fmt.Errorf("failed to set hostname: %w", err)
				}
				fmt.Printf("Hostname set to %s\n", hostname)
				return nil
			},
		},
		{
			Name:  "tls",
			Usage: "Toggle use of TLS for secure connections",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				// get
				useTLS, err := config.Get[bool](ctx, "useTLS")
				if err != nil {
					return fmt.Errorf("failed to get useTLS: %w", err)
				}
				// set
				if err := config.Set(ctx, "useTLS", !useTLS); err != nil {
					return fmt.Errorf("failed to set useTLS: %w", err)
				}
				fmt.Printf("useTLS set to %t\n", !useTLS)
				return nil
			},
		},
		{
			Name:  "tls-paths",
			Usage: "Set paths for TLS key and certificate `<key path> <cert path>`",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				tlsKeyPath := cmd.Args().Get(0)
				tlsCertPath := cmd.Args().Get(1)

				// set paths in config
				if err := config.Set(ctx, "tlsKeyPath", tlsKeyPath); err != nil {
					return fmt.Errorf("failed to set tlsKeyPath: %w", err)
				}
				if err := config.Set(ctx, "tlsCertPath", tlsCertPath); err != nil {
					return fmt.Errorf("failed to set tlsCertPath: %w", err)
				}
				fmt.Printf("TLS paths set: key=%s, cert=%s\n", tlsKeyPath, tlsCertPath)
				return nil
			},
		},
		{
			Name:  "log",
			Usage: "Set the log level `<level>` (levels: none, error, warn, info, debug)",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				level := cmd.Args().Get(0)
				if err := config.Set(ctx, "logLevel", level); err != nil {
					return fmt.Errorf("failed to set log level: %w", err)
				}
				fmt.Printf("Log level set to %s\n", level)
				return nil
			},
		},
		{
			Name:  "backup-pass",
			Usage: "Set the backup encryption password `<password>`",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				password := cmd.Args().Get(0)
				if err := config.Set(ctx, "backupPassword", password); err != nil {
					return fmt.Errorf("failed to set backup password: %w", err)
				}
				fmt.Println("Backup password set")
				return nil
			},
		},
		{
			Name:  "add-admin",
			Usage: "Add a user ID to the admin list `<user ID>`",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				userID := cmd.Args().Get(0)
				if userID == "" {
					return fmt.Errorf("user ID cannot be empty")
				}
				adminUserIDs, err := config.Get[[]string](ctx, "adminUserIDs")
				if err != nil {
					return fmt.Errorf("failed to get adminUserIDs: %w", err)
				}
				// Add user ID to the list if not already present
				for _, id := range adminUserIDs {
					if id == userID {
						fmt.Println("User ID already in admin list")
						return nil
					}
				}
				adminUserIDs = append(adminUserIDs, userID)
				if err := config.Set(ctx, "adminUserIDs", adminUserIDs); err != nil {
					return fmt.Errorf("failed to set adminUserIDs: %w", err)
				}
				fmt.Printf("Admin user ID %s added\n", userID)
				return nil
			},
		},
		{
			Name:  "bot-token",
			Usage: "Set the bot token `<token>`",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				token := cmd.Args().Get(0)
				if token == "" {
					return fmt.Errorf("bot token cannot be empty")
				}
				if err := config.Set(ctx, "botToken", token); err != nil {
					return fmt.Errorf("failed to set bot token: %w", err)
				}
				fmt.Println("Bot token set")
				return nil
			},
		},
	},
}
