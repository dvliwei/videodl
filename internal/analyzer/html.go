package analyzer

import (
	"encoding/json"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type HTMLMediaInfo struct {
	PageTitle  string
	Candidates []MediaRef
}

type MediaRef struct {
	URL        string
	DisplayURL string
	SourceTag  string
	MIMEType   string
	Title      string
}

func ExtractMediaFromHTML(htmlContent string, baseURL *url.URL) HTMLMediaInfo {
	info := HTMLMediaInfo{}

	if strings.TrimSpace(htmlContent) == "" {
		return info
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return info
	}

	resolvedBase := resolveBase(doc, baseURL)
	info.PageTitle = extractTitle(doc)

	seen := map[string]bool{}
	var refs []MediaRef

	walk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}

		switch n.Data {
		case "video", "audio":
			for _, a := range n.Attr {
				if a.Key == "src" {
					ref := buildRef(a.Val, resolvedBase, n.Data, attr(n, "type"), "")
					if ref != nil && !seen[ref.DisplayURL] {
						seen[ref.DisplayURL] = true
						refs = append(refs, *ref)
					}
				}
			}
		case "source":
			parent := n.Parent
			if parent != nil && (parent.Data == "video" || parent.Data == "audio") {
				src := attr(n, "src")
				if src != "" {
					ref := buildRef(src, resolvedBase, "source", attr(n, "type"), "")
					if ref != nil && !seen[ref.DisplayURL] {
						seen[ref.DisplayURL] = true
						refs = append(refs, *ref)
					}
				}
			}
		case "meta":
			prop := attr(n, "property")
			name := attr(n, "name")
			content := attr(n, "content")
			isVideoMeta := content != "" && (prop == "og:video" ||
				prop == "og:video:url" ||
				prop == "og:video:secure_url" ||
				name == "twitter:player" ||
				name == "video")
			isVideoTypeMeta := prop == "og:video:type" && content != ""
			if isVideoMeta {
				ref := buildRef(content, resolvedBase, "meta", "", "")
				if ref != nil && !seen[ref.DisplayURL] {
					seen[ref.DisplayURL] = true
					refs = append(refs, *ref)
				}
			} else if isVideoTypeMeta {
				updateLastRefMIME(refs, content)
			}
		case "script":
			if attr(n, "type") == "application/ld+json" {
				extractJSONLDMedia(n, resolvedBase, seen, &refs)
			}
		}
	})

	info.Candidates = refs
	return info
}

func extractTitle(doc *html.Node) string {
	var tagTitle, ogTitle, metaTitle string

	walk(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch n.Data {
		case "title":
			tagTitle = textContent(n)
		case "meta":
			prop := attr(n, "property")
			name := attr(n, "name")
			content := attr(n, "content")
			if prop == "og:title" && ogTitle == "" {
				ogTitle = content
			}
			if name == "twitter:title" && metaTitle == "" {
				metaTitle = content
			}
		}
	})

	if strings.TrimSpace(tagTitle) != "" {
		return strings.TrimSpace(tagTitle)
	}
	if strings.TrimSpace(ogTitle) != "" {
		return strings.TrimSpace(ogTitle)
	}
	return strings.TrimSpace(metaTitle)
}

func resolveBase(doc *html.Node, fallback *url.URL) *url.URL {
	var baseHref string
	walk(doc, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "base" {
			baseHref = attr(n, "href")
		}
	})
	if baseHref == "" {
		return fallback
	}

	base, err := url.Parse(baseHref)
	if err != nil {
		return fallback
	}
	if base.IsAbs() {
		return base
	}
	if fallback != nil {
		return fallback.ResolveReference(base)
	}
	return base
}

func buildRef(rawURL string, base *url.URL, sourceTag, mime, title string) *MediaRef {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}

	lower := strings.ToLower(rawURL)
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "#") {
		return nil
	}

	var absolute *url.URL
	var err error
	if base != nil {
		absolute, err = base.Parse(rawURL)
	} else {
		absolute, err = url.Parse(rawURL)
	}
	if err != nil || absolute == nil {
		return nil
	}
	if absolute.Scheme != "http" && absolute.Scheme != "https" {
		return nil
	}
	if absolute.Host == "" && absolute.Path == "" {
		return nil
	}
	if strings.Contains(absolute.Path, " ") {
		return nil
	}
	if strings.Contains(rawURL, " ") && !strings.Contains(absolute.Path, "%20") {
		return nil
	}

	displayURL := *absolute
	displayURL.RawQuery = ""
	displayURL.Fragment = ""
	displayURL.RawPath = ""

	return &MediaRef{
		URL:        absolute.String(),
		DisplayURL: displayURL.String(),
		SourceTag:  sourceTag,
		MIMEType:   mime,
		Title:      title,
	}
}

func extractJSONLDMedia(n *html.Node, base *url.URL, seen map[string]bool, refs *[]MediaRef) {
	jsonText := textContent(n)
	if jsonText == "" {
		return
	}

	var raw interface{}
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return
	}

	urls := collectJSONLDMediaURLs(raw)
	for _, u := range urls {
		ref := buildRef(u, base, "jsonld", "", "")
		if ref != nil && !seen[ref.DisplayURL] {
			seen[ref.DisplayURL] = true
			*refs = append(*refs, *ref)
		}
	}
}

func collectJSONLDMediaURLs(v interface{}) []string {
	var urls []string
	switch x := v.(type) {
	case map[string]interface{}:
		if schemaType, ok := x["@type"].(string); ok {
			t := strings.ToLower(schemaType)
			if strings.Contains(t, "video") || strings.Contains(t, "audio") || strings.Contains(t, "music") {
				if s, ok := x["url"].(string); ok {
					urls = append(urls, s)
				}
			}
		}
		for key, val := range x {
			lowerKey := strings.ToLower(key)
			if lowerKey == "contenturl" || lowerKey == "embedurl" {
				if s, ok := val.(string); ok {
					urls = append(urls, s)
				}
			}
			urls = append(urls, collectJSONLDMediaURLs(val)...)
		}
	case []interface{}:
		for _, item := range x {
			urls = append(urls, collectJSONLDMediaURLs(item)...)
		}
	}
	return urls
}

func updateLastRefMIME(refs []MediaRef, mime string) {
	if len(refs) == 0 {
		return
	}
	for i := len(refs) - 1; i >= 0; i-- {
		if refs[i].SourceTag == "meta" && refs[i].MIMEType == "" {
			refs[i].MIMEType = mime
			return
		}
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	var buf strings.Builder
	var visit func(*html.Node)
	visit = func(cur *html.Node) {
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				buf.WriteString(c.Data)
			} else if c.Type == html.ElementNode {
				visit(c)
			}
		}
	}
	visit(n)
	return buf.String()
}

func walk(doc *html.Node, fn func(*html.Node)) {
	if doc == nil {
		return
	}
	fn(doc)
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}
