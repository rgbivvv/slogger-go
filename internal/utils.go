package internal

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// SyncTree copies src into dst, skipping files with matching size whose
// destination mtime is at least as new as the source. Removes dest entries
// that no longer exist under src.
func SyncTree(src, dst string) error {
	seen := make(map[string]struct{})
	if err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		seen[rel] = struct{}{}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if sameFile(info, target) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	}); err != nil {
		return err
	}
	return filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dst, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if _, ok := seen[rel]; ok {
			return nil
		}
		if info.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			return filepath.SkipDir
		}
		return os.Remove(path)
	})
}

func sameFile(srcInfo os.FileInfo, dstPath string) bool {
	dstInfo, err := os.Stat(dstPath)
	if err != nil || dstInfo.IsDir() {
		return false
	}
	return srcInfo.Size() == dstInfo.Size() && !srcInfo.ModTime().After(dstInfo.ModTime())
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
