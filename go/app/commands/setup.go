package commands

import (
	"context"
	"fmt"
	"os/exec"
	"sprout/go/app"
	"sprout/go/platform/database"
	"sprout/go/platform/database/config"
	"sprout/go/platform/x"
	"time"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/Data-Corruption/stdx/xterm/prompt"
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

			cfgDBI, ok := a.DB.GetDBis()[database.ConfigDBIName]
			if !ok {
				return fmt.Errorf("failed to get config DBI from app DB")
			}

			err := a.DB.Update(func(txn *lmdb.Txn) error {
				// read general settings
				key := []byte("generalSettings")
				var settings config.GeneralSettings
				if err := database.GetAndUnmarshal(txn, cfgDBI, key, &settings); err != nil {
					if !lmdb.IsNotFound(err) {
						return fmt.Errorf("failed to get general settings: %w", err)
					}
				}

				// bot token
				token, err := prompt.String("")
				if err != nil || token == "" {
					return fmt.Errorf("failed to read bot token: %w", err)
				}
				settings.BotToken = token

				x.Typewrite("\nGreat, now your discord user ID.\nYou can get it by enabling dev mode in discord and right clicking your name)\n", 25)

				// admin ID
				adminID, err := prompt.String("")
				if err != nil || adminID == "" {
					return fmt.Errorf("failed to read admin ID: %w", err)
				}
				settings.AdminWhitelist = append(settings.AdminWhitelist, adminID)

				// write updated settings
				if err := database.MarshalAndPut(txn, cfgDBI, key, &settings); err != nil {
					return fmt.Errorf("failed to store general settings: %w", err)
				}
				return nil
			})
			if err == nil {

				// write update followup to register commands on restart
				rc := config.RestartContext{RegisterCmds: true}
				if err := config.Set(a.Config, "restartContext", &rc); err != nil {
					return fmt.Errorf("failed to set update followup in config: %w", err)
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
			}
			return err
		},
	}
})

func mysteriousThinking() {
	for i := 0; i < 3; i++ {
		fmt.Print(".")
		time.Sleep(700 * time.Millisecond)
	}
}
