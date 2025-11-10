package xhtml

import (
	"fmt"
	"net/http"

	"golang.org/x/net/html"
)

// Fetch fetches the HTML document from the specified URL
func Fetch(url string) (*html.Node, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := html.Parse(res.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// FindElementByTag recursively searches for an element with the specified tag name. Returns the first matching element found.
func FindElementByTag(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := FindElementByTag(c, tag); result != nil {
			return result
		}
	}

	return nil
}

// FindElementByID recursively searches for an element with the specified id. Returns the first matching element found.
func FindElementByID(n *html.Node, id string) *html.Node {
	if GetAttribute(n, "id") == id {
		return n
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := FindElementByID(c, id); result != nil {
			return result
		}
	}

	return nil
}

// GetLiElements returns all li elements in the specified ul element
func GetLiElements(ul *html.Node) []*html.Node {
	var lis []*html.Node
	for c := ul.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "li" {
			lis = append(lis, c)
		}
	}
	return lis
}

// GetAttribute returns the value of a specific attribute of an HTML node
func GetAttribute(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
