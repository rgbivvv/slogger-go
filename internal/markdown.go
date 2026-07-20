package internal

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

// NewMarkdown returns a goldmark converter that preserves raw HTML
// (audio, video, source, etc.) in post content.
func NewMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
}
