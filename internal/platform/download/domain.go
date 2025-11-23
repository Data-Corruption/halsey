package download

import (
	"net/url"
	"strings"
)

// Domain represents the domain of the media URL.
type Domain int

const (
	DomainUnknown Domain = iota
	DomainInstagram
	DomainReddit
	DomainXitter
	DomainYouTube
	DomainYoutubeShorts
)

func (d Domain) String() string {
	switch d {
	case DomainInstagram:
		return "instagram"
	case DomainReddit:
		return "reddit"
	case DomainXitter:
		return "xitter"
	case DomainYouTube:
		return "youtube"
	case DomainYoutubeShorts:
		return "youtube_shorts"
	default:
		return "unknown"
	}
}

// ParseDomain determines the domain from the given raw URL.
func ParseDomain(rawURL string) Domain {
	switch {
	case hasAnyPrefix(rawURL, []string{
		"https://www.instagram.com/",
		"https://m.instagram.com/",
		"https://instagram.com/",
		"https://www.instagr.am/",
		"https://m.instagr.am/",
		"https://instagr.am/"}):
		return DomainInstagram
	case hasAnyPrefix(rawURL, []string{
		"https://www.reddit.com/",
		"https://reddit.com/",
		"https://v.redd.it/",
		"https://i.redd.it/",
		"https://www.redd.it/",
		"https://np.reddit.com/",
		"https://amp.reddit.com/",
		"https://m.reddit.com/",
		"https://old.reddit.com/",
		"https://new.reddit.com/"}):
		return DomainReddit
	case hasAnyPrefix(rawURL, []string{
		"https://x.com/",
		"https://www.x.com/",
		"https://mobile.x.com/",
		"https://twitter.com/",
		"https://www.twitter.com/",
		"https://mobile.twitter.com/",
		"https://t.co/"}):
		return DomainXitter
	case hasAnyPrefix(rawURL, []string{
		"https://www.youtube.com/",
		"https://youtube.com/",
		"https://youtu.be/"}):
		if hasAnyPrefix(rawURL, []string{
			"https://youtube.com/shorts/",
			"https://www.youtube.com/shorts/"}) {
			return DomainYoutubeShorts
		}
		return DomainYouTube
	default:
		return DomainUnknown
	}
}

// IsSingleValidURL checks if the given string contains a single valid URL.
func IsSingleValidURL(s string) bool {
	// fast path
	if !(strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")) {
		return false
	}
	// fuzzy check
	count := strings.Count(s, "http://") + strings.Count(s, "https://")
	if count != 1 || strings.ContainsAny(s, " \t\n") {
		return false
	}
	// parse URL
	u, err := url.ParseRequestURI(s)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
