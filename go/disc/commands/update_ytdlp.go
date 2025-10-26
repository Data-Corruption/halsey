package commands

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

const (
	updateYtdlpTimeout = 30 * time.Second
	updateYtdlpMaxOut  = 800 // max output characters to keep
)

var updateYtdlpCommand = BotCommand{
	IsGlobal:     true,
	RequireAdmin: true,
	FilterBots:   true,
	Data: discord.SlashCommandCreate{
		Name:        "update-ytdlp",
		Description: "Attempt to update yt-dlp to the latest version.",
	},
	Handler: func(ctx context.Context, event *events.ApplicationCommandInteractionCreate) error {
		xlog.Debug(ctx, "Update ytdlp command called")
		if err := resDeferMessage(ctx, event); err != nil {
			return fmt.Errorf("error deferring interaction: %s", err)
		}
		code, output, err := updateYtdlp()
		var content string
		if err != nil {
			content = fmt.Sprintf("yt-dlp update failed with code %d:\n```\n%s\n```", code, output)
		} else {
			content = fmt.Sprintf("yt-dlp updated successfully to the latest version! Output:\n```\n%s\n```", output)
		}
		fMsg, err := resFollowupMessage(ctx, event, discord.NewMessageCreateBuilder().
			SetContent(content).SetEphemeral(true).Build())
		if err != nil {
			return fmt.Errorf("error sending followup message: %s", err)
		}
		xlog.Info(ctx, "yt-dlp update command completed")
		_ = fMsg
		return nil
	},
}

func init() {
	List["update-ytdlp"] = updateYtdlpCommand
}

var ytdlpUpdateMutex = sync.Mutex{}

func updateYtdlp() (int, string, error) {
	ytdlpUpdateMutex.Lock()
	defer ytdlpUpdateMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), updateYtdlpTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-m", "pip", "install", "-U", "yt-dlp[default]")

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()

	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			code = -2 // indicate timeout distinctly
			err = fmt.Errorf("command timed out after 30s")
		} else {
			code = -1 // generic exec failure
		}
	}

	out := buf.String()
	if len(out) > updateYtdlpMaxOut {
		out = out[len(out)-updateYtdlpMaxOut:]
	}

	return code, out, err
}
