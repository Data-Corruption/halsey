package listeners

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sprout/internal/app"
	"sprout/internal/discord/chat"
	"sprout/internal/discord/emojis"
	"sprout/internal/discord/externallinks"
	"sprout/internal/discord/response"
	"sprout/internal/platform/database"
	"sprout/internal/platform/download"
	"sprout/internal/platform/download/extractors"
	"sprout/pkg/compressor"
	"sprout/pkg/workqueue"
	"strings"
	"sync"
	"time"

	"github.com/Data-Corruption/lmdb-go/lmdb"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func OnGuildMessageCreate(a *app.App, event *events.GuildMessageCreate) {
	a.DiscordWG.Add(1) // track for graceful shutdown

	// acquire semaphore
	select {
	case a.DiscordEventLimiter <- struct{}{}:
	default:
		a.DiscordWG.Done()
		a.Log.Warn("Event limiter reached, dropping guild message create")
		return
	}

	go func() {
		defer a.DiscordWG.Done()
		defer func() { <-a.DiscordEventLimiter }()

		message, err := a.Client.Rest.GetMessage(event.Message.ChannelID, event.Message.ID)
		if err != nil {
			a.Log.Error("Failed to get message: ", err)
			return
		}

		a.Log.Info("Received message: ", message.Content)

		handleAiChat(a, event, message)

		handleExternalLinks(a, event, message)
	}()
}

func handleAiChat(a *app.App, event *events.GuildMessageCreate, message *discord.Message) {
	// skip auto expand output, that will be added by auto expand code
	if isAutoExpandOutput(message) {
		a.Log.Debugf("Skipping auto-expand output message: %s", message.ID)
		return
	}
	// add message to chat
	msg := chat.ParseUserMessage(message, a.Client)
	a.Chat.UpsertChannelMessages(message.ChannelID, event.GuildID, func(buf []chat.Message) []chat.Message {
		if msg.Role == "assistant" {
			// skip if already in buf
			if slices.ContainsFunc(buf, func(m chat.Message) bool {
				return m.ID == msg.ID
			}) {
				a.Log.Debugf("Bot message already in chat: %s", msg.ID)
				return buf
			}
		}
		a.Log.Debugf("Added message to chat: %s", msg.ID)
		return append(buf, msg)
	})
}

func isAutoExpandOutput(msg *discord.Message) bool {
	for _, c := range msg.Components {
		discordActionRow, ok := c.(discord.ActionRowComponent)
		if !ok {
			continue
		}
		for _, comp := range discordActionRow.Components {
			if linkButton, ok := comp.(discord.ButtonComponent); ok {
				if (linkButton.Style == discord.ButtonStyleLink) && ((linkButton.Emoji != nil) || (linkButton.Label == "▲")) {
					return true
				}
			}
		}
	}
	return false
}

const ConfirmLengthThreshold = 1200 // 20 minutes

var autoExpandDomains = []download.Domain{
	download.DomainReddit,
	download.DomainYoutubeShorts,
	download.DomainRedGifs,
}

func handleExternalLinks(a *app.App, event *events.GuildMessageCreate, message *discord.Message) {
	if message.Author.Bot {
		return
	}

	guild, err := database.ViewGuild(a.DB, event.GuildID)
	if err != nil {
		a.Log.Error("Failed to get guild: ", err)
		return
	}
	if !guild.AntiRotEnabled {
		return
	}

	user, err := database.ViewUser(a.DB, message.Author.ID)
	if err != nil {
		a.Log.Error("Failed to get user: ", err)
		return
	}

	// extract anti-rot supported links into slice.
	links := externallinks.ExtractLinks(message)
	if len(links) == 0 {
		return
	}

	// get config
	cfg, err := database.ViewConfig(a.DB)
	if err != nil {
		a.Log.Error("Failed to get config: ", err)
		return
	}

	// download them, update DB references.
	for i := 0; i < len(links); i++ {
		link := links[i]
		if i > 20 {
			a.Log.Warnf("Too many links in message: %s", message.ID)
			break
		}

		// check if it's already in the db
		_, err := database.ViewAsset(a.DB, link.Url)
		if err == nil {
			a.Log.Debugf("Asset already in db: %s", link.Url)
			continue
		}
		if !lmdb.IsNotFound(err) {
			a.Log.Error("Failed to get asset: ", err)
			continue
		}

		switch link.Domain {
		case download.DomainReddit:
			if cfg.DisableAutoExpand.Reddit {
				continue
			}

			if a.RedditQueue.Has(link.Url) {
				a.Log.Debugf("Reddit link already in queue: %s", link.Url)
				continue
			}

			// extract
			var result any
			var errMsg string
			var err error
			exWG := &sync.WaitGroup{}
			exWG.Add(1)
			a.RedditQueue.Enqueue(link.Url, false, func() error {
				defer exWG.Done()
				result, errMsg, err = extractors.Reddit(a.Context, link.Url, a.UserAgent)
				return err
			})
			exWG.Wait()
			if err != nil {
				msg := fmt.Sprintf("Error extracting %s: %s", link.Url, errMsg)
				if _, err := response.MessageBotChannel(a, event.GuildID, discord.NewMessageCreateBuilder().SetContent(msg).Build()); err != nil {
					a.Log.Error("Failed to message bot channel: ", err)
				}
				continue
			}
			a.Log.Debugf("Extracted %s: %v", link.Url, result)

			runDwld := func(url string) (string, error) {
				var path string
				var err error
				dwldWG := &sync.WaitGroup{}
				dwldWG.Add(1)
				a.RedditQueue.Enqueue(link.Url, false, func() error {
					defer dwldWG.Done()
					path, err = download.DownloadMedia(url, a.TempDir, a.UserAgent, time.Second*10)
					return err
				})
				dwldWG.Wait()
				return path, err
			}

			// download
			switch r := result.(type) {
			case extractors.RedditTextResult:
				continue
			case extractors.RedditLinkResult:
				linkDomain := download.ParseDomain(r.Url)
				if linkDomain == download.DomainRedGifs {
					// append to the links slice
					links = append(links, externallinks.Link{
						Url:         r.Url,
						Domain:      download.DomainRedGifs,
						CrossSrcUrl: link.Url,
					})
					continue
				}
				continue // maybe do redgifs check
			case extractors.RedditBasicResult:
				path, err := runDwld(r.Url)
				if err != nil {
					a.Log.Error("Failed to download: ", err)
					continue
				}
				if err := externallinks.AddAsset(a, link.Url, path); err != nil {
					a.Log.Error("Failed to add asset: ", err)
					continue
				}
				continue
			case extractors.RedditVideoResult:
				path, err := runDwld(r.Url)
				if err != nil {
					a.Log.Error("Failed to download: ", err)
					continue
				}
				if err := externallinks.AddAsset(a, link.Url, path); err != nil {
					a.Log.Error("Failed to add asset: ", err)
					continue
				}
				continue
			case extractors.RedditGalleryResult:
				continue // TODO: wip
			default:
				a.Log.Errorf("Unknown Reddit result type: %T", result)
				continue
			}
		case download.DomainYouTube, download.DomainYoutubeShorts, download.DomainRedGifs:
			var targetQueue *workqueue.Queue
			if link.Domain == download.DomainYouTube {
				if cfg.DisableAutoExpand.YouTube {
					continue
				}
				targetQueue = a.YoutubeQueue
				// get length
				var seconds int
				var err error
				wg := &sync.WaitGroup{}
				wg.Add(1)
				a.YoutubeQueue.Enqueue(link.Url, false, func() error {
					seconds, err = download.YtDLPLength(a.Context, link.Url, 10*time.Second)
					wg.Done()
					return err
				})
				wg.Wait()
				if err != nil {
					a.Log.Error("Failed to get length: ", err)
					continue
				}
				// if too long, prompt admin for confirmation
				if seconds > ConfirmLengthThreshold {
					msgBuilder := discord.NewMessageCreateBuilder()
					msgBuilder.AddComponents(discord.NewActionRow(
						discord.NewSecondaryButton("✖", "download.deny"),
						discord.NewSuccessButton("✔", "download.confirm"),
					))
					// link if first field
					msgBuilder.SetContentf("%s is %d seconds long. Confirm download?", link.Url, seconds)
					if _, err := response.MessageBotChannel(a, event.GuildID, msgBuilder.Build()); err != nil {
						a.Log.Error("Failed to message bot channel: ", err)
					}
					continue
				}
			} else if link.Domain == download.DomainYoutubeShorts {
				if cfg.DisableAutoExpand.YouTubeShorts {
					continue
				}
				targetQueue = a.YoutubeQueue
			} else {
				if cfg.DisableAutoExpand.RedGifs {
					continue
				}
				targetQueue = a.RedGifsQueue
			}

			// download
			var path string
			var err error
			wg := &sync.WaitGroup{}
			wg.Add(1)
			targetQueue.Enqueue(link.Url, false, func() error {
				path, err = download.YtDLP(a.Context, link.Url, a.TempDir, 30*time.Second)
				wg.Done()
				return err
			})
			wg.Wait()
			if err != nil {
				a.Log.Error("Failed to download: ", err)
				continue
			}

			// add asset
			ogSrcUrl := link.Url
			if link.CrossSrcUrl != "" {
				ogSrcUrl = link.CrossSrcUrl
			}
			if err := externallinks.AddAsset(a, ogSrcUrl, path); err != nil {
				a.Log.Error("Failed to add asset: ", err)
				continue
			}
		}
	}

	// if message is lone link and user has this domain enabled for auto expand, perform auto expand.
	soloLink := strings.TrimSpace(message.Content)
	if download.IsSingleValidURL(soloLink) {
		// if not reddit, shorts, or redgifs, return.
		domain := download.ParseDomain(soloLink)
		if !slices.Contains(autoExpandDomains, domain) {
			return
		}
		// if user has this domain disabled for auto expand, return.
		switch domain {
		case download.DomainReddit:
			if !user.AutoExpand.Reddit || cfg.DisableAutoExpand.Reddit {
				return
			}
		case download.DomainYoutubeShorts:
			if !user.AutoExpand.YouTubeShorts || cfg.DisableAutoExpand.YouTubeShorts {
				return
			}
		case download.DomainRedGifs:
			if !user.AutoExpand.RedGifs || cfg.DisableAutoExpand.RedGifs {
				return
			}
		}
		asset, err := database.ViewAsset(a.DB, soloLink)
		if err != nil {
			if !lmdb.IsNotFound(err) {
				a.Log.Error("Failed to get asset: ", err)
			}
			return
		}
		if err := expandAsset(a, event.GuildID, soloLink, asset, guild, message); err != nil {
			a.Log.Error("Failed to expand asset: ", err)
			return
		}
	}
}

func expandAsset(a *app.App, guildID snowflake.ID, url string, asset *database.Asset, guild *database.Guild, message *discord.Message) error {
	// create temp dir
	tempDir, err := os.MkdirTemp(a.TempDir, "")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// copy asset to temp dir
	inPath := filepath.Join(tempDir, filepath.Base(asset.Path))
	if err := copyFile(asset.Path, inPath); err != nil {
		return fmt.Errorf("failed to copy asset: %w", err)
	}

	if filepath.Ext(asset.Path) == ".tar" {
		return nil // TODO: wip
	}

	// get type
	mediaType := compressor.MediaTypeFromExt(filepath.Ext(asset.Path))
	if mediaType == "" {
		return fmt.Errorf("failed to get media type: %s", filepath.Ext(asset.Path))
	}

	// compress it, skip if it's a gif or webp
	var outPath string
	if mediaType == "gif" || mediaType == "webp" {
		outPath = inPath
	} else {
		baseName := strings.TrimSuffix(filepath.Base(asset.Path), filepath.Ext(asset.Path))
		switch mediaType {
		case compressor.MediaTypeVideo:
			outPath = filepath.Join(tempDir, "out"+baseName+".webm")
			if err := a.Compressor.Video(a.Context, inPath, outPath, 5*time.Minute); err != nil {
				return fmt.Errorf("failed to compress video: %w", err)
			}
		case compressor.MediaTypeImage:
			outPath = filepath.Join(tempDir, "out"+baseName+".avif")
			if err := a.Compressor.Image(a.Context, inPath, outPath, 30*time.Second); err != nil {
				return fmt.Errorf("failed to compress image: %w", err)
			}
		default:
			return fmt.Errorf("failed to get media type: %s", mediaType)
		}
	}

	// open file
	file, err := os.Open(outPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			a.Log.Error("Failed to close file: ", err)
		}
	}()

	// check size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	if fileInfo.Size() <= 0 {
		return fmt.Errorf("file is empty: %s", outPath)
	}

	// get upload size limit
	var uploadSizeLimit int64
	switch guild.PremiumTier {
	case discord.PremiumTier2:
		uploadSizeLimit = 49 * 1024 * 1024 // 49mb
	case discord.PremiumTier3:
		uploadSizeLimit = 99 * 1024 * 1024 // 99mb
	default:
		uploadSizeLimit = 24 * 1024 * 1024 // 24mb
	}

	if fileInfo.Size() > uploadSizeLimit {
		return fmt.Errorf("file is too big: %s (%d bytes)", outPath, fileInfo.Size())
	}

	// create external link btn
	var externBtn discord.ActionRowComponent
	smEmoji, ok := emojis.GetSocialMediaEmoji(a, download.ParseDomain(url))
	if !ok {
		externBtn = discord.NewActionRow(discord.NewLinkButton("▲", url)) // fallback
	} else {
		externBtn = discord.NewActionRow(discord.NewLinkButton("", url).WithEmoji(discord.NewCustomComponentEmoji(smEmoji.ID)))
	}

	// message channel / upload file
	a.Log.Debugf("Uploading file %s (%d bytes)", outPath, fileInfo.Size())
	aName := filepath.Base(outPath)
	aeOut, err := a.Client.Rest.CreateMessage(message.ChannelID, discord.NewMessageCreateBuilder().
		SetFlags(discord.MessageFlagIsComponentsV2).
		AddComponents(
			externBtn,
			discord.NewMediaGallery(
				discord.MediaGalleryItem{Media: discord.UnfurledMediaItem{URL: "attachment://" + aName}},
			),
			discord.NewTextDisplayf("`%s` • <t:%d:f>", message.Author.Username, message.CreatedAt.Unix()),
		).
		AddFile(aName, "", file).
		Build())
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// delete user message
	if err := a.Client.Rest.DeleteMessage(message.ChannelID, message.ID); err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	// update ai chat buffer
	// replace user message with tombstone, add system message for auto-expand output
	domain := download.ParseDomain(url)
	domainName := "Unknown"
	switch domain {
	case download.DomainReddit:
		domainName = "Reddit"
	case download.DomainYoutubeShorts:
		domainName = "YouTubeShorts"
	case download.DomainRedGifs:
		domainName = "RedGifs"
	}

	a.Chat.UpsertChannelMessages(message.ChannelID, guildID, func(buf []chat.Message) []chat.Message {
		// replace original user message with tombstone
		for i := range buf {
			if buf[i].ID == message.ID {
				buf[i] = chat.Message{
					ID:      message.ID,
					Role:    "system",
					Content: fmt.Sprintf("[tombstone][id=%s][author=%s] Link-only message auto-expanded then deleted.", message.ID, message.Author.Username),
					Created: message.CreatedAt,
				}
				break
			}
		}

		// add system message for auto-expand output
		return append(buf, chat.Message{
			ID:      aeOut.ID,
			Role:    "system",
			Content: fmt.Sprintf("[id=%s][auto_expand_from=%s][attachment] Uploaded media mirror. Original %s link attached.", aeOut.ID, message.ID, domainName),
			Created: aeOut.CreatedAt,
		})
	})

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
