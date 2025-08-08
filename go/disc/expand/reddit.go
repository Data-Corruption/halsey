package expand

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/storage/config"
	"halsey/go/xhtml"
	"halsey/go/xnet"
	"os"
	"path/filepath"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"golang.org/x/net/html"
)

type TextScrapeResult struct{}
type BasicScrapeResult struct {
	Path string // Path to the downloaded media file in the default temp directory.
}
type LinkScrapeResult struct {
	Url string // Could be anything, but usually a link to an external site, often news.
}
type GalleryScrapeResult struct {
	Paths []string // Paths to the downloaded media files in the default temp directory.
}

func reddit(ctx context.Context, sourceMessage *discord.Message, url string) error {
	// create status message
	statusMsg, err := createStatusMessage(ctx, sourceMessage, "Downloading Reddit Post...")
	if err != nil {
		return fmt.Errorf("failed to create status message: %w", err)
	}

	// scrape reddit post
	res, publicErr, err := scrapeRedditPost(ctx, sourceMessage, url)
	if err != nil {
		failStatusMessage(ctx, statusMsg, publicErr, 0)
		return fmt.Errorf("failed to scrape reddit post: %w", err)
	}

	switch v := res.(type) {
	case TextScrapeResult:
		return disc.Client.Rest.DeleteMessage(statusMsg.ChannelID, statusMsg.ID)
	case BasicScrapeResult:
		updateStatusMessage(ctx, statusMsg, "Uploading Media...")

		// get upload size limit
		uploadSizeLimitMB, err := config.Get[uint](ctx, "uploadSizeLimitMB")
		if err != nil {
			failStatusMessage(ctx, statusMsg, "Internal error: failed to get upload size limit", 0)
			return fmt.Errorf("failed to get upload size limit: %w", err)
		}
		uploadSizeLimit := int64(uploadSizeLimitMB-2) * 1024 * 1024 // Convert MB to bytes

		// open file
		file, err := os.Open(v.Path)
		if err != nil {
			failStatusMessage(ctx, statusMsg, "Internal error: failed to open file", 0)
			return fmt.Errorf("failed to open file %s: %w", v.Path, err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				xlog.Errorf(ctx, "failed to close file %s: %s", v.Path, err)
			}
			if err := os.Remove(v.Path); err != nil {
				xlog.Errorf(ctx, "failed to remove file %s: %s", v.Path, err)
			}
		}()

		// check size
		fileInfo, err := file.Stat()
		if err != nil {
			failStatusMessage(ctx, statusMsg, "Internal error: failed to get file info", 0)
			return fmt.Errorf("failed to get file info for %s: %w", v.Path, err)
		}
		if fileInfo.Size() <= 0 {
			failStatusMessage(ctx, statusMsg, "Internal error: Downloaded file is empty", 0)
			return fmt.Errorf("downloaded file %s is empty", v.Path)
		}
		if fileInfo.Size() > uploadSizeLimit {
			xlog.Debugf(ctx, "File %s is too big (%d bytes), not uploading", v.Path, fileInfo.Size())
			return failStatusMessage(ctx, statusMsg, fmt.Sprintf("File is too big (%d bytes), not uploading", fileInfo.Size()), 5*time.Second)
		}

		// message channel / upload file
		xlog.Debugf(ctx, "Uploading file %s (%d bytes)", v.Path, fileInfo.Size())
		aName := filepath.Base(v.Path)
		if _, err := disc.Client.Rest.CreateMessage(sourceMessage.ChannelID, discord.NewMessageCreateBuilder().
			SetFlags(discord.MessageFlagIsComponentsV2).
			AddComponents(
				discord.NewActionRow(discord.NewLinkButton("Open In Reddit", url)),
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
	case LinkScrapeResult:
		// create a message with the expanded URL
		_, err := disc.Client.Rest.CreateMessage(sourceMessage.ChannelID, discord.NewMessageCreateBuilder().
			AddActionRow(discord.NewLinkButton("Open In Reddit", url)).
			SetContentf("%s\n`%s` • <t:%d:f>", v.Url, sourceMessage.Author.Username, sourceMessage.CreatedAt.Unix()).
			Build())
		if err != nil {
			return fmt.Errorf("failed to create message: %w", err)
		}
		// delete status and source message only if everything went well
		if err := disc.Client.Rest.DeleteMessage(statusMsg.ChannelID, statusMsg.ID); err != nil {
			xlog.Errorf(ctx, "failed to delete status message: %s", err)
		}
		return disc.Client.Rest.DeleteMessage(sourceMessage.ChannelID, sourceMessage.ID)
	case GalleryScrapeResult:
		return disc.Client.Rest.DeleteMessage(statusMsg.ChannelID, statusMsg.ID)
	default:
		failStatusMessage(ctx, statusMsg, "Internal error: unknown scrape result type", 0)
		return fmt.Errorf("unknown scrape result type: %T", res)
	}
}

// scrapeRedditPost scrapes reddit urls for the main media content, downloads them, and returns the file paths.
// Returns result, public safe error message, and error if any.
func scrapeRedditPost(ctx context.Context, statusMsg *discord.Message, url string) (any, string, error) {
	xlog.Debugf(ctx, "Scraping Reddit URL: %s", url)
	doc := &html.Node{}
	shredditPost := &html.Node{}
	contentHref, postType := "", ""
	// loop exists for crosspost type
	link := url
	for {
		// get page from url
		var err error
		if doc, err = xhtml.Fetch(link); err != nil {
			return nil, "Failed to fetch Reddit page", fmt.Errorf("failed to fetch Reddit page: %w", err)
		}
		// get post element
		shredditPost = xhtml.FindElementByTag(doc, "shreddit-post")
		if shredditPost == nil {
			return nil, "No shreddit-post element found", fmt.Errorf("no shreddit-post element found in document")
		}
		// get attributes
		contentHref = xhtml.GetAttribute(shredditPost, "content-href")
		if contentHref == "" {
			return nil, "No content-href attribute found", fmt.Errorf("no content-href attribute found in shreddit-post element")
		}
		postType = xhtml.GetAttribute(shredditPost, "post-type")
		if postType == "" {
			return nil, "No post-type attribute found", fmt.Errorf("no post-type attribute found in shreddit-post element")
		}
		// crosspost / link types
		if postType == "crosspost" {
			link = "https://www.reddit.com" + contentHref
			continue
		}
		break
	}
	// handle post types
	switch postType {
	case "text":
		xlog.Debugf(ctx, "Found text post: %s", contentHref)
		return TextScrapeResult{}, "", nil
	case "link":
		xlog.Debugf(ctx, "Found link post: %s", contentHref)
		return LinkScrapeResult{Url: contentHref}, "", nil
	case "image", "gif":
		xlog.Debugf(ctx, "Found %s media: %s", postType, contentHref)
		updateStatusMessage(ctx, statusMsg, fmt.Sprintf("Downloading %s media...", postType))
		if outPath, err := xnet.DownloadMedia(contentHref, 5*time.Minute); err != nil {
			return nil, "Failed to download media", fmt.Errorf("failed to download media: %w", err)
		} else {
			return BasicScrapeResult{Path: outPath}, "", nil
		}
	case "video":
		shredditPlayer := xhtml.FindElementByTag(doc, "shreddit-player")
		if shredditPlayer == nil {
			shredditPlayer = xhtml.FindElementByTag(doc, "shreddit-player-2")
			if shredditPlayer == nil {
				return nil, "Post type is video but no shreddit-player or shreddit-player-2 element found", fmt.Errorf("post type is video but no shreddit-player or shreddit-player-2 element found")
			}
		}
		src := xhtml.GetAttribute(shredditPlayer, "src")
		if src == "" {
			return nil, "No src attribute found in shreddit-player or shreddit-player-2 element", fmt.Errorf("no src attribute found in shreddit-player or shreddit-player-2 element")
		}
		xlog.Debugf(ctx, "Found video media: %s", src)
		updateStatusMessage(ctx, statusMsg, "Downloading video...")
		if outPath, err := xnet.DownloadMedia(src, 5*time.Minute); err != nil {
			return nil, "Failed to download media", fmt.Errorf("failed to download media: %w", err)
		} else {
			return BasicScrapeResult{Path: outPath}, "", nil
		}
	case "gallery":
		return GalleryScrapeResult{}, "", nil
		/* skipping gallery for now, might finish later
		output := GalleryScrapeResult{Paths: []string{}}
		carousel := xhtml.FindElementByTag(doc, "gallery-carousel")
		if carousel == nil {
			return nil, "Post type is gallery but no gallery-carousel element found", fmt.Errorf("post type is gallery but no gallery-carousel element found")
		}
		ul := xhtml.FindElementByTag(carousel, "ul")
		if ul == nil {
			return nil, "Post type is gallery but no ul element found in gallery-carousel", fmt.Errorf("post type is gallery but no ul element found in gallery-carousel")
		}
		lis := xhtml.GetLiElements(ul)
		if len(lis) == 0 {
			return nil, "Post type is gallery but no li elements found in ul element of gallery-carousel", fmt.Errorf("post type is gallery but no li elements found in ul element of gallery-carousel")
		}
		xlog.Debugf(ctx, "Downloading %d items from gallery", len(lis))
		updateStatusMessage(ctx, statusMsg, true, fmt.Sprintf("Downloading %d items from gallery...", len(lis)))
		for index, li := range lis {
			img := xhtml.FindElementByTag(li, "img")
			if img == nil {
				return nil, "Post type is gallery but no img element found in li element %d of ul element of gallery-carousel", fmt.Errorf("post type is gallery but no img element found in li element %d of ul element of gallery-carousel", index)
			}
			src := xhtml.GetAttribute(img, "src")
			if src == "" {
				src = xhtml.GetAttribute(img, "data-lazy-src")
				if src == "" {
					return nil, "Could not determine the src of gallery-carousel element [%d]", fmt.Errorf("could not determine the src of gallery-carousel element [%d]", index)
				}
			}
			xlog.Debugf(ctx, "Found gallery media: %s", src)
			if outPath, err := xnet.DownloadMedia(src); err != nil {
				return nil, "Failed to download media", fmt.Errorf("failed to download media: %w", err)
			} else {
				output.Paths = append(output.Paths, outPath)
			}
		}
		return output, "", nil
		*/
	default:
		return nil, "Unsupported post type: '%s'", fmt.Errorf("unsupported post type: '%s'", postType)
	}
}
