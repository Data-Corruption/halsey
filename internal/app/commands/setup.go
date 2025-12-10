package commands

import (
	"context"
	"fmt"
	"os/exec"
	"sprout/internal/app"
	"sprout/internal/platform/database"
	"sprout/pkg/x"
	"time"

	"github.com/Data-Corruption/stdx/xterm/prompt"
	"github.com/disgoorg/snowflake/v2"
	"github.com/urfave/cli/v3"
)

var Setup = register(func(a *app.App) *cli.Command {
	return &cli.Command{
		Name:  "setup",
		Usage: "setup the bot",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			mysteriousThinking()
			x.Typewrite(" hmm, this is an interesting system. please give me a moment ", 25)
			mysteriousThinking()

			fmt.Printf("\n\nLogger initialized.\n")
			time.Sleep(500 * time.Millisecond)
			fmt.Printf("Database initialized.\n")
			time.Sleep(250 * time.Millisecond)
			fmt.Printf("Configuration loaded.\n\n")
			time.Sleep(1000 * time.Millisecond)

			x.Typewrite("Hello, It's a pleasure to meet you, I'm Halsey\n", 25)
			x.Typewrite("When you're ready, enter your bot token\n", 25)

			// get bot token
			token, err := prompt.String("")
			if err != nil || token == "" {
				return fmt.Errorf("failed to read bot token: %w", err)
			}

			x.Typewrite("\nGreat, now your discord user ID.\nYou can get it by enabling dev mode in discord and right clicking your name)\n", 25)

			// get bootstrap admin ID
			idStr, err := prompt.String("")
			if err != nil || idStr == "" {
				return fmt.Errorf("failed to read admin ID: %w", err)
			}
			userID, err := snowflake.Parse(idStr)
			if err != nil {
				return fmt.Errorf("failed to parse admin ID as snowflake: %w", err)
			}

			// Write them both to the database

			if err := database.UpdateConfig(a.DB, func(cfg *database.Configuration) error {
				cfg.BotToken = token
				cfg.RestartCtx.RegisterCmds = true // likely first run, ensure commands are registered
				return nil
			}); err != nil {
				return fmt.Errorf("failed to update notification setting in config: %w", err)
			}

			if _, err := database.UpsertUser(a.DB, userID, func(user *database.User) error {
				user.IsAdmin = true
				return nil
			}); err != nil {
				return fmt.Errorf("failed to set user %d as admin: %w", userID, err)
			}

			if a.Version == "vX.X.X" {
				x.Typewrite("\nDevelopment build detected, skipping restart\n", 25)
				return nil
			}

			x.Typewrite("\nLooks good, restarting now, you should see me get on discord in a moment\n", 25)
			a.SetPostCleanup(func() error {
				iCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(iCtx, "systemctl", "--user", "restart", "halsey.service") // fine since not an update / migration
				if out, err := cmd.CombinedOutput(); err != nil {
					return fmt.Errorf("failed to restart service: %v, output: %s", err, string(out))
				}
				return nil
			})

			return nil
		},
	}
})

func mysteriousThinking() {
	for i := 0; i < 3; i++ {
		fmt.Print(".")
		time.Sleep(700 * time.Millisecond)
	}
}
