package expand

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/disc/respond"
	"halsey/go/storage/config"
	"halsey/go/xnet"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
)

var lastytdlpWasErr int64 = 0

func youtube(ctx context.Context, sourceMessage *discord.Message, url string) error {
	// load state from config
	enabled, err := config.Get[bool](ctx, "expandYouTube")
	if err != nil {
		return fmt.Errorf("failed to get expandYouTube from config: %w", err)
	}
	if !enabled {
		xlog.Debugf(ctx, "YouTube expansion is disabled, skipping")
		return nil
	}

	// create status message
	statusMsg, err := createStatusMessage(ctx, sourceMessage, "Downloading Media...")
	if err != nil {
		return fmt.Errorf("failed to create status message: %w", err)
	}

	// get upload size limit
	uploadSizeLimitMB, err := config.Get[uint](ctx, "uploadSizeLimitMB")
	if err != nil {
		failStatusMessage(ctx, statusMsg, "Internal error: failed to get upload size limit", 0)
		return fmt.Errorf("failed to get upload size limit: %w", err)
	}
	uploadSizeLimit := int64(uploadSizeLimitMB-2) * 1024 * 1024 // Convert MB to bytes

	// download media
	outPath, err := xnet.DownloadMedia(url, 5*time.Minute)
	if err != nil {
		swapped := atomic.CompareAndSwapInt64(&lastytdlpWasErr, 0, 1)
		if !swapped {
			// if failed to swap that means the last download also failed. Disable further downloads for now and msg bot channel
			xlog.Debugf(ctx, "Last ytdlp download also failed, skipping further downloads")
			err := config.Set(ctx, "expandYouTube", false)
			if err != nil {
				xlog.Errorf(ctx, "failed to disable youtube expansion after repeated ytdlp failures: %s", err)
			}
			respond.BotChannel(ctx, "YouTube expansion has been automatically disabled due to repeated download failures. I still can't auto update ytdlp.. *unintellegible gibberish*")
		}
		failStatusMessage(ctx, statusMsg, "Internal error: failed to download media", 0)
		return fmt.Errorf("failed to download media: %w", err)
	}

	// reset last error flag
	atomic.StoreInt64(&lastytdlpWasErr, 0)

	// open file
	file, err := os.Open(outPath)
	if err != nil {
		failStatusMessage(ctx, statusMsg, "Internal error: failed to open file", 0)
		return fmt.Errorf("failed to open file %s: %w", outPath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			xlog.Errorf(ctx, "failed to close file %s: %s", outPath, err)
		}
		if err := os.Remove(outPath); err != nil {
			xlog.Errorf(ctx, "failed to remove file %s: %s", outPath, err)
		}
	}()

	// check size
	fileInfo, err := file.Stat()
	if err != nil {
		failStatusMessage(ctx, statusMsg, "Internal error: failed to get file info", 0)
		return fmt.Errorf("failed to get file info for %s: %w", outPath, err)
	}
	if fileInfo.Size() <= 0 {
		failStatusMessage(ctx, statusMsg, "Internal error: Downloaded file is empty", 0)
		return fmt.Errorf("downloaded file %s is empty", outPath)
	}
	if fileInfo.Size() > uploadSizeLimit {
		xlog.Debugf(ctx, "File %s is too big (%d bytes), not uploading", outPath, fileInfo.Size())
		return failStatusMessage(ctx, statusMsg, fmt.Sprintf("File is too big (%d bytes), not uploading", fileInfo.Size()), 5*time.Second)
	}

	// message channel / upload file
	xlog.Debugf(ctx, "Uploading file %s (%d bytes)", outPath, fileInfo.Size())
	aName := filepath.Base(outPath)
	if _, err := disc.Client.Rest.CreateMessage(sourceMessage.ChannelID, discord.NewMessageCreateBuilder().
		SetFlags(discord.MessageFlagIsComponentsV2).
		AddComponents(
			discord.NewActionRow(discord.NewLinkButton("Open In Youtube", url)),
			discord.NewMediaGallery(
				discord.MediaGalleryItem{Media: discord.UnfurledMediaItem{URL: "attachment://" + aName}},
			),
			discord.NewTextDisplayf("`%s` • <t:%d:f>", sourceMessage.Author.Username, sourceMessage.CreatedAt.Unix()),
		).
		AddFile(aName, "", file).
		Build()); err != nil {
		failStatusMessage(ctx, statusMsg, "Internal error: failed to create message", 0)
		return fmt.Errorf("failed to create message: %w", err)
	}

	// delete status and source message only if everything went well
	if err := disc.Client.Rest.DeleteMessage(statusMsg.ChannelID, statusMsg.ID); err != nil {
		xlog.Errorf(ctx, "failed to delete status message: %s", err)
	}
	return disc.Client.Rest.DeleteMessage(sourceMessage.ChannelID, sourceMessage.ID)
}
