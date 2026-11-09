package internal

import (
	"regexp"
	"strings"
)

var (
	imgTagRe     = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	mediaTagRe   = regexp.MustCompile(`(?i)<(?:video|audio)\b[^>]*>`)
	loadingAttr  = regexp.MustCompile(`(?i)\sloading\s*=`)
	decodingAttr = regexp.MustCompile(`(?i)\sdecoding\s*=`)
	preloadAttr  = regexp.MustCompile(`(?i)\spreload\s*=`)
)

// LightenMedia defers media downloads in rendered post HTML: images get
// loading="lazy" decoding="async", and video/audio get preload="none".
// Attributes the author already set are left alone.
func LightenMedia(htmlContent string) string {
	htmlContent = imgTagRe.ReplaceAllStringFunc(htmlContent, func(tag string) string {
		var add string
		if !loadingAttr.MatchString(tag) {
			add += ` loading="lazy"`
		}
		if !decodingAttr.MatchString(tag) {
			add += ` decoding="async"`
		}
		return insertAttrs(tag, add)
	})
	return mediaTagRe.ReplaceAllStringFunc(htmlContent, func(tag string) string {
		if preloadAttr.MatchString(tag) {
			return tag
		}
		return insertAttrs(tag, ` preload="none"`)
	})
}

func insertAttrs(tag, attrs string) string {
	if attrs == "" {
		return tag
	}
	end := len(tag) - 1 // ">"
	if strings.HasSuffix(tag, "/>") {
		end--
		for end > 0 && tag[end-1] == ' ' {
			end--
		}
	}
	return tag[:end] + attrs + tag[end:]
}
