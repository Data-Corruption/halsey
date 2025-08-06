package expand

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/storage/assets"
	"halsey/go/storage/config"
	"halsey/go/xnet"
	"os"
	"path/filepath"

	"github.com/Data-Corruption/stdx/xhttp"
	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
)

func youtube(ctx context.Context, sourceMessage *discord.Message, url string) error {
	// create status message
	sp := disc.GetSpinnerEmoji(ctx)
	statusMsg, err := disc.Client.Rest.CreateMessage(sourceMessage.ChannelID, discord.NewMessageCreateBuilder().
		SetMessageReferenceByID(sourceMessage.ID).
		SetContent(sp+" Downloading Media... ").
		Build())
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// get upload size limit
	uploadSizeLimitMB, err := config.Get[uint](ctx, "uploadSizeLimitMB")
	if err != nil {
		updateStatusMessage(ctx, statusMsg, false, "Internal error: failed to get upload size limit")
		return fmt.Errorf("failed to get upload size limit: %w", err)
	}
	uploadSizeLimit := int64(uploadSizeLimitMB-2) * 1024 * 1024 // Convert MB to bytes

	outPath, err := xnet.DownloadMedia(url)
	if err != nil {
		updateStatusMessage(ctx, statusMsg, false, "Internal error: failed to download media")
		return fmt.Errorf("failed to download media: %w", err)
	}

	updateStatusMessage(ctx, statusMsg, true, "Uploading...")
	file, err := os.Open(outPath)
	if err != nil {
		updateStatusMessage(ctx, statusMsg, false, "Internal error: failed to open file")
		return fmt.Errorf("failed to open file %s: %w", outPath, err)
	}
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		os.Remove(outPath)
		updateStatusMessage(ctx, statusMsg, false, "Internal error: failed to get file info")
		return fmt.Errorf("failed to get file info for %s: %w", outPath, err)
	}
	if fileInfo.Size() <= 0 {
		file.Close()
		// os.Remove(outPath)
		updateStatusMessage(ctx, statusMsg, false, "Internal error: Media file is empty")
		return fmt.Errorf("media file %s is empty", outPath)
	}

	// if too big, close and add to self hosted assets, otherwise defer cleanup and upload
	if fileInfo.Size() > uploadSizeLimit {
		xlog.Debugf(ctx, "File %s is too big (%d bytes), adding to self-hosted assets", outPath, fileInfo.Size())
		// get hostname
		hostname, err := config.Get[string](ctx, "hostname")
		if err != nil {
			file.Close()
			os.Remove(outPath)
			updateStatusMessage(ctx, statusMsg, false, "Internal error: failed to get hostname")
			return fmt.Errorf("failed to get hostname: %w", err)
		}
		if hostname == "" {
			xlog.Debug(ctx, "Hostname is empty, aborting self-hosted asset upload")
			file.Close()
			os.Remove(outPath)
			updateStatusMessage(ctx, statusMsg, false, "Media exceeds size limit and bot is not configured to self-host assets")
			return nil
		}
		// get useTLS
		useTLS, err := config.Get[bool](ctx, "useTLS")
		if err != nil {
			file.Close()
			os.Remove(outPath)
			updateStatusMessage(ctx, statusMsg, false, "Internal error: failed to get useTLS")
			return fmt.Errorf("failed to get useTLS: %w", err)
		}
		// get server
		server, ok := ctx.Value("httpServer").(*xhttp.Server)
		if !ok {
			file.Close()
			os.Remove(outPath)
			updateStatusMessage(ctx, statusMsg, false, "Internal error: failed to get http server")
			return fmt.Errorf("failed to get http server: %w", err)
		}

		file.Close()
		name, err := assets.Add(ctx, file.Name())
		os.Remove(outPath) // remove temp file
		if err != nil {
			updateStatusMessage(ctx, statusMsg, false, "Internal error: failed to add file to assets")
			return fmt.Errorf("failed to add file %s to assets: %w", file.Name(), err)
		}

		hURL := "http"
		if useTLS {
			hURL += "s"
		}
		hURL += "://" + hostname + server.Addr() + "/a/" + name
		xlog.Debugf(ctx, "File %s added to assets, URL: %s", outPath, hURL)

		// create message with asset link
		if _, err := disc.Client.Rest.CreateMessage(sourceMessage.ChannelID, discord.NewMessageCreateBuilder().
			SetFlags(discord.MessageFlagIsComponentsV2).
			AddComponents(
				discord.NewActionRow(discord.NewLinkButton("Open In Youtube", url)),
				discord.NewMediaGallery(
					discord.MediaGalleryItem{Media: discord.UnfurledMediaItem{URL: hURL}},
				),
				discord.NewTextDisplayf("`%s` • <t:%d:f>", sourceMessage.Author.Username, sourceMessage.CreatedAt.Unix()),
			).
			Build()); err != nil {
			updateStatusMessage(ctx, statusMsg, false, "Internal error: failed to create message")
			return fmt.Errorf("failed to create message: %w", err)
		}
	} else {
		xlog.Debugf(ctx, "File %s is within size limit (%d bytes), uploading directly", outPath, fileInfo.Size())
		defer func() {
			if err := file.Close(); err != nil {
				xlog.Errorf(ctx, "failed to close file %s: %s", outPath, err)
			}
			if err := os.Remove(outPath); err != nil {
				xlog.Errorf(ctx, "failed to remove file %s: %s", outPath, err)
			}
		}()
		// upload file
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
			updateStatusMessage(ctx, statusMsg, false, "Internal error: failed to create message")
			return fmt.Errorf("failed to create message: %w", err)
		}
	}

	// delete status and source message
	if err := disc.Client.Rest.DeleteMessage(statusMsg.ChannelID, statusMsg.ID); err != nil {
		xlog.Errorf(ctx, "failed to delete status message: %s", err)
	}
	return disc.Client.Rest.DeleteMessage(sourceMessage.ChannelID, sourceMessage.ID)
}
