package internal

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

func WipeDirFilesOnly(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		return os.Remove(path)
	})
}

func PruneStaleHTML(buildDir string, keep map[string]struct{}) error {
	entries, err := os.ReadDir(buildDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".html" {
			continue
		}
		if _, ok := keep[name]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(buildDir, name)); err != nil {
			return err
		}
	}
	return nil
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
var multiSep = regexp.MustCompile(`_+`)

// Slugify lowercases, strips non-ASCII, and replaces non-alnum with _.
// ponytail: no NFKD (needs x/text); ASCII titles are fine, add NFKD if needed.
func Slugify(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r < unicode.MaxASCII {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	s := nonAlnum.ReplaceAllString(b.String(), "_")
	s = strings.Trim(s, "_")
	return multiSep.ReplaceAllString(s, "_")
}
