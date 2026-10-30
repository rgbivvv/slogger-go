package internal

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

const (
	descriptionMaxLen = 160
	ogTitleMaxLen     = 70
)

var (
	firstImgSrc = regexp.MustCompile(`(?is)<img\b[^>]*?\bsrc\s*=\s*["']([^"']+)["']`)
	scriptBlock = regexp.MustCompile(`(?is)<script\b.*?</script>`)
	styleBlock  = regexp.MustCompile(`(?is)<style\b.*?</style>`)
	blockTag    = regexp.MustCompile(`(?i)</?(p|div|br|hr|h[1-6]|li|ul|ol|dt|dd|dl|blockquote|pre|table|tr|td|th|figure|figcaption|section|article|header|footer)\b[^>]*>`)
	anyTag      = regexp.MustCompile(`(?s)<[^>]*>`)
)

// PostDescription returns the leading plain text of rendered post HTML,
// cut at a word boundary to roughly descriptionMaxLen characters.
func PostDescription(htmlContent string) string {
	return excerpt(htmlContent, descriptionMaxLen)
}

// excerpt returns the leading plain text of rendered HTML, cut at a word
// boundary to roughly maxLen characters.
func excerpt(htmlContent string, maxLen int) string {
	s := scriptBlock.ReplaceAllString(htmlContent, " ")
	s = styleBlock.ReplaceAllString(s, " ")
	s = blockTag.ReplaceAllString(s, " ")
	s = anyTag.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(html.UnescapeString(s)), " ")
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	cut := string(runes[:maxLen])
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:-") + "…"
}

// PostImage returns an absolute URL for the first <img> in rendered post
// HTML, or "" if the post has none.
func PostImage(htmlContent, siteURL string) string {
	m := firstImgSrc.FindStringSubmatch(htmlContent)
	if m == nil {
		return ""
	}
	src := html.UnescapeString(m[1])
	switch {
	case strings.HasPrefix(src, "data:"):
		return ""
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		return src
	case strings.HasPrefix(src, "//"):
		return "https:" + src
	}
	return strings.TrimRight(siteURL, "/") + "/" + strings.TrimLeft(strings.TrimPrefix(src, "./"), "/")
}

// PostMetaTags builds Open Graph and Twitter card tags for link previews.
func PostMetaTags(p Post, siteName string) string {
	var b strings.Builder
	tag := func(attr, key, val string) {
		if val == "" {
			return
		}
		fmt.Fprintf(&b, "<meta %s=\"%s\" content=\"%s\">\n", attr, key, html.EscapeString(val))
	}
	tag("property", "og:type", "article")
	tag("property", "og:site_name", siteName)
	title := p.OGTitle
	if title == "" {
		title = p.Title
	}
	tag("property", "og:title", title)
	tag("property", "og:url", p.Permalink)
	tag("property", "og:description", p.Description)
	tag("name", "description", p.Description)
	if p.Image != "" {
		tag("property", "og:image", p.Image)
		tag("name", "twitter:card", "summary_large_image")
	} else {
		tag("name", "twitter:card", "summary")
	}
	return b.String()
}
