package internal

import (
	"github.com/microcosm-cc/bluemonday"
)

func SanitizePolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements(
		"a", "abbr", "acronym", "b", "blockquote", "code", "em", "i", "li", "ol",
		"strong", "ul", "br", "p", "span", "small", "hr", "pre",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"table", "thead", "tbody", "tr", "th", "td",
		"img", "audio", "video", "source",
	)
	p.AllowAttrs("class").Globally()
	p.AllowAttrs("href", "title").OnElements("a")
	p.AllowAttrs("src", "alt").OnElements("img")
	p.AllowAttrs("src", "controls").OnElements("audio", "video")
	p.AllowAttrs("src", "type").OnElements("source")
	p.AllowURLSchemes("http", "https")
	return p
}

func SanitizeHTML(html string, p *bluemonday.Policy) string {
	return p.Sanitize(html)
}
