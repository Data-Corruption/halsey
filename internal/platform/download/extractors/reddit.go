package extractors

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sprout/internal/platform/download"
	"sprout/pkg/xhtml"
	"strings"
	"time"

	"github.com/Data-Corruption/stdx/xlog"
	"golang.org/x/net/html"
)

// TODO: switch to .json since that is probably better for all parties involved

type RedditTextResult struct{}
type RedditLinkResult struct{ Url string }       // URL of the external link
type RedditBasicResult struct{ Url string }      // URL of an image or gif file
type RedditVideoResult struct{ Url string }      // URL of a video file
type RedditGalleryResult struct{ Urls []string } // URLs of multiple image files

// Reddit extracts and returns the main media content urls from reddit posts.
// Includes a chan for status updates to be sent to the user.
// Returns result, user safe error message, and an error if any.
func Reddit(ctx context.Context, rawURL, userAgent string) (any, string, error) {
	xlog.Debugf(ctx, "Extracting media from Reddit URL: %s", rawURL)

	// resolve short URLs
	url := resolveShortRedditUrl(rawURL, userAgent)
	if url != rawURL {
		xlog.Debugf(ctx, "Resolved short Reddit URL: %s -> %s", rawURL, url)
	}

	doc := &html.Node{}
	shredditPost := &html.Node{}
	contentHref, postType := "", ""

	// get post info (loop to handle crossposts)
	i := 0
	for {
		// get page with 10s timeout
		type fetchResult struct {
			doc *html.Node
			err error
		}
		fetchCh := make(chan fetchResult, 1)
		go func() {
			d, e := xhtml.Fetch(url)
			fetchCh <- fetchResult{d, e}
		}()
		select {
		case res := <-fetchCh:
			if res.err != nil {
				return nil, "Failed to fetch Reddit page", fmt.Errorf("failed to fetch Reddit page: %w", res.err)
			}
			doc = res.doc
		case <-time.After(10 * time.Second):
			return nil, "Timed out fetching Reddit page", fmt.Errorf("timed out fetching Reddit page after 10s")
		}

		// get post element
		shredditPost = xhtml.FindElementByTag(doc, "shreddit-post")
		if shredditPost == nil {
			return nil, "No shreddit-post element found", fmt.Errorf("no shreddit-post element found in document")
		}

		// TODO: check if post was removed

		// get attributes
		contentHref = xhtml.GetAttribute(shredditPost, "content-href")
		if contentHref == "" {
			return nil, "No content-href attribute found", fmt.Errorf("no content-href attribute found in shreddit-post element")
		}
		postType = xhtml.GetAttribute(shredditPost, "post-type")
		if postType == "" {
			return nil, "No post-type attribute found", fmt.Errorf("no post-type attribute found in shreddit-post element")
		}

		// handle crossposts
		if postType == "crosspost" {
			if i >= 5 {
				return nil, "Umm... crosspost chain suspiciously long. Aborting.", fmt.Errorf("crosspost chain too long")
			}
			url = "https://www.reddit.com" + contentHref
			xlog.Debugf(ctx, "Following crosspost to: %s", url)
			i++
			continue
		}
		// handle link based crossposts
		if postType == "link" {
			linkDomain := download.ParseDomain(contentHref)
			if linkDomain == download.DomainReddit {
				if i >= 5 {
					return nil, "Umm... crosspost chain suspiciously long. Aborting.", fmt.Errorf("crosspost chain too long")
				}
				url = contentHref
				xlog.Debugf(ctx, "Following crosspost to: %s", url)
				i++
				continue
			}
		}

		break
	}

	// handle post types
	switch postType {
	case "text":
		xlog.Debugf(ctx, "Found text post: %s", contentHref)
		return RedditTextResult{}, "", nil
	case "link":
		xlog.Debugf(ctx, "Found link post: %s", contentHref)
		return RedditLinkResult{Url: contentHref}, "", nil
	case "image", "gif":
		xlog.Debugf(ctx, "Found %s media: %s", postType, contentHref)
		return RedditBasicResult{Url: contentHref}, "", nil
	case "video":
		shredditPlayer := xhtml.FindElementByTag(doc, "shreddit-player")
		if shredditPlayer == nil {
			shredditPlayer = xhtml.FindElementByTag(doc, "shreddit-player-2")
			if shredditPlayer == nil {
				errMsg := "post type is video but no shreddit-player or shreddit-player-2 element found"
				return nil, errMsg, errors.New(errMsg)
			}
		}
		src := xhtml.GetAttribute(shredditPlayer, "src")
		if src == "" {
			return nil, "No src attribute found in shreddit-player or shreddit-player-2 element", fmt.Errorf("no src attribute found in shreddit-player or shreddit-player-2 element")
		}
		xlog.Debugf(ctx, "Found video media: %s", src)
		return RedditVideoResult{Url: src}, "", nil
	case "gallery":
		output := RedditGalleryResult{Urls: []string{}}

		carousel := xhtml.FindElementByTag(doc, "gallery-carousel")
		if carousel == nil {
			errMsg := "post type is gallery but no gallery-carousel element found"
			return nil, errMsg, errors.New(errMsg)
		}

		ul := xhtml.FindElementByTag(carousel, "ul")
		if ul == nil {
			errMsg := "post type is gallery but no ul element found in gallery-carousel"
			return nil, errMsg, errors.New(errMsg)
		}
		lis := xhtml.GetLiElements(ul)
		if len(lis) == 0 {
			errMsg := "post type is gallery but no li elements found in ul element of gallery-carousel"
			return nil, errMsg, errors.New(errMsg)
		}

		xlog.Debugf(ctx, "Found %d items in gallery", len(lis))
		for index, li := range lis {
			img := xhtml.FindElementByTag(li, "img")
			if img == nil {
				errMsg := fmt.Sprintf("post type is gallery but no img element found in li element %d of ul element of gallery-carousel", index)
				return nil, errMsg, errors.New(errMsg)
			}

			src := xhtml.GetAttribute(img, "src")
			if src == "" {
				src = xhtml.GetAttribute(img, "data-lazy-src")
				if src == "" {
					errMsg := fmt.Sprintf("could not determine the src of gallery-carousel element [%d]", index)
					return nil, errMsg, errors.New(errMsg)
				}
			}

			xlog.Debugf(ctx, "Found gallery media: %s", src)
			output.Urls = append(output.Urls, src)
		}

		return output, "", nil
	default:
		return nil, "Unsupported post type: '%s'", fmt.Errorf("unsupported post type: '%s'", postType)
	}
}

// resolveShortRedditUrl resolves reddit short share URLs like /s/<id>
func resolveShortRedditUrl(rawURL, userAgent string) string {
	if !strings.Contains(rawURL, "/s/") {
		return rawURL
	}
	req, _ := http.NewRequest("HEAD", rawURL, nil)
	req.Header.Set("User-Agent", userAgent)
	client := &http.Client{
		Timeout: 10 * time.Second,
		// don't auto-follow; we want the Location header
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return rawURL
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if loc != "" {
			if strings.HasPrefix(loc, "/") {
				loc = "https://www.reddit.com" + loc
			}

			// drop query/fragment tracking to avoid share shells
			pu, err := url.Parse(loc)
			if err != nil {
				return loc
			}

			// strip share/utm noise and unify host
			pu.RawQuery, pu.Fragment = "", ""
			switch strings.ToLower(pu.Host) {
			case "reddit.com", "m.reddit.com", "old.reddit.com":
				pu.Host = "www.reddit.com"
			}

			// decode each segment up to twice, then re-encode once
			ep := pu.EscapedPath() // preserves existing escapes
			segs := strings.Split(ep, "/")
			decSegs := make([]string, len(segs))
			encSegs := make([]string, len(segs))
			for i, s := range segs {
				if s == "" {
					continue
				}
				t := s
				for j := 0; j < 2; j++ { // collapse double literal, e.g. %25xx -> %xx
					d, err := url.PathUnescape(t)
					if err != nil || d == t {
						break
					}
					t = d
				}
				decSegs[i] = t
				encSegs[i] = url.PathEscape(t) // single, correct encoding
			}
			pu.Path = strings.Join(decSegs, "/")    // decoded (for callers)
			pu.RawPath = strings.Join(encSegs, "/") // encoded (for HTTP)

			return pu.String()
		}
	}
	return rawURL
}
