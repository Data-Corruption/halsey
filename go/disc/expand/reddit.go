package expand

import (
	"context"
	"fmt"
	"halsey/go/disc"
	"halsey/go/xhtml"
	"halsey/go/xnet"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Data-Corruption/stdx/xlog"
	"github.com/disgoorg/disgo/discord"
	"golang.org/x/net/html"
)

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
	sp := disc.GetSpinnerEmoji(ctx)
	statusMsg, err := disc.Client.Rest.CreateMessage(sourceMessage.ChannelID, discord.NewMessageCreateBuilder().
		SetMessageReferenceByID(sourceMessage.ID).
		SetContent(sp+" Parsing Reddit Post... ").
		Build())
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// scrape reddit post
	res, publicErr, err := scrapeRedditPost(ctx, sourceMessage, url)
	if err != nil {
		updateStatusMessage(ctx, statusMsg, publicErr)
		return fmt.Errorf("failed to scrape reddit post: %w", err)
	}

	// TODO: get upload size limit

	switch v := res.(type) {
	case BasicScrapeResult:
		updateStatusMessage(ctx, statusMsg, "Finished downloading media.")
	case LinkScrapeResult:
		updateStatusMessage(ctx, statusMsg, "Found link.")
	case GalleryScrapeResult:
		updateStatusMessage(ctx, statusMsg, fmt.Sprintf("Found gallery with %d items.", len(v.Paths)))
	default:
		updateStatusMessage(ctx, statusMsg, "Unknown media type.")
	}

	return nil
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
	case "link":
		xlog.Debugf(ctx, "Found link post: %s", contentHref)
		return LinkScrapeResult{Url: contentHref}, "", nil
	case "image", "gif":
		xlog.Debugf(ctx, "Found %s media: %s", postType, contentHref)
		updateStatusMessage(ctx, statusMsg, fmt.Sprintf("Downloading %s media...", postType))
		if outPath, err := xnet.DownloadMedia(contentHref); err != nil {
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
		if outPath, err := xnet.DownloadMedia(src); err != nil {
			return nil, "Failed to download media", fmt.Errorf("failed to download media: %w", err)
		} else {
			return BasicScrapeResult{Path: outPath}, "", nil
		}
	case "gallery":
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
		updateStatusMessage(ctx, statusMsg, fmt.Sprintf("Downloading %d items from gallery...", len(lis)))
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
	default:
		return nil, "Unsupported post type: '%s'", fmt.Errorf("unsupported post type: '%s'", postType)
	}
}

// CompressImage compresses an image to meet the target file size.
func CompressImage(ctx context.Context, input string, targetSize int64) (string, error) {
	newName := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input)) + ".jpg"
	output := filepath.Join(filepath.Dir(input), newName)

	// Iterate over quality values (2 is highest quality, 31 is lowest).
	for quality := 2; quality <= 31; quality++ {
		cmd := exec.Command("ffmpeg", "-i", input, "-q:v", fmt.Sprintf("%d", quality), output, "-y")
		if err := cmd.Run(); err != nil {
			xlog.Debugf(ctx, "Failed to run ffmpeg command: %v", cmd)
			return "", fmt.Errorf("failed to run ffmpeg: %w", err)
		}
		// Check the size of the output file.
		fileInfo, err := os.Stat(output)
		if err != nil {
			return "", fmt.Errorf("failed to stat output file: %w", err)
		}
		if fileInfo.Size() <= targetSize {
			return output, nil // success
		}
	}
	// If no suitable compression was found, remove the output file to avoid clutter.
	os.Remove(output)
	return "", fmt.Errorf("failed to compress image to target size %d bytes", targetSize)
}

/*
// if the image is too large, compress it
		if maxSizeBytes > 0 {
			if fi, err := os.Stat(outPath); err != nil {
				return nil, "Failed to get file info", fmt.Errorf("failed to get file info: %w", err)
			} else if fi.Size() > maxSizeBytes {
				xlog.Debugf(ctx, "Downloaded media (%dMB) exceeds max upload size (%dMB)", fi.Size()/(1024*1024), maxSizeBytes/(1024*1024))
				// compress image
				var newOutPath string
				if newOutPath, err = CompressImage(ctx, outPath, maxSizeBytes/2); err != nil {
					return nil, "", err
				} else {
					os.Remove(outPath)
					return []ScrapeResult{{Path: newOutPath}}, "", nil
				}
			}
		}
*/
