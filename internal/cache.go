package internal

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// ponytail: mtime+size only; content-hash if clock skew or editors rewrite in place without mtime bump.
type cachedPost struct {
	MtimeNano   int64  `json:"mtime_nano"`
	Size        int64  `json:"size"`
	FnameSrc    string `json:"fname_src"`
	Fname       string `json:"fname"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Epoch       int64  `json:"epoch"`
	Date        string `json:"date"`
	HTMLContent string `json:"html_content"`
	Permalink   string `json:"permalink"`
}

func (c cachedPost) toPost() Post {
	return Post{
		FnameSrc:    c.FnameSrc,
		Fname:       c.Fname,
		Slug:        c.Slug,
		Title:       c.Title,
		Epoch:       c.Epoch,
		Date:        c.Date,
		HTMLContent: c.HTMLContent,
		Permalink:   c.Permalink,
		PageCached:  true,
	}
}

func postToCached(p Post, mtimeNano, size int64) cachedPost {
	return cachedPost{
		MtimeNano:   mtimeNano,
		Size:        size,
		FnameSrc:    p.FnameSrc,
		Fname:       p.Fname,
		Slug:        p.Slug,
		Title:       p.Title,
		Epoch:       p.Epoch,
		Date:        p.Date,
		HTMLContent: p.HTMLContent,
		Permalink:   p.Permalink,
	}
}

func cacheFilePath(cacheDir, srcName string) string {
	return filepath.Join(cacheDir, srcName+".json")
}

func loadCachedPost(cacheDir, srcName string) (cachedPost, bool) {
	data, err := os.ReadFile(cacheFilePath(cacheDir, srcName))
	if err != nil {
		return cachedPost{}, false
	}
	var c cachedPost
	if err := json.Unmarshal(data, &c); err != nil {
		return cachedPost{}, false
	}
	return c, true
}

func pageCachePath(cacheDir, srcName string) string {
	return filepath.Join(cacheDir, "pages", srcName+".html")
}

func cachedPageExists(cacheDir, srcName string) bool {
	_, err := os.Stat(pageCachePath(cacheDir, srcName))
	return err == nil
}

func saveCachedPage(cacheDir, srcName, srcPath string) error {
	if err := os.MkdirAll(filepath.Join(cacheDir, "pages"), 0o755); err != nil {
		return err
	}
	return copyFile(srcPath, pageCachePath(cacheDir, srcName))
}

func copyCachedPage(cacheDir, srcName, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return copyFile(pageCachePath(cacheDir, srcName), destPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func saveCachedPost(cacheDir, srcName string, c cachedPost) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFilePath(cacheDir, srcName), data, 0o644)
}

func pruneCache(cacheDir string, alive map[string]struct{}) error {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		src := name[:len(name)-len(".json")]
		if _, ok := alive[src]; ok {
			continue
		}
		_ = os.Remove(filepath.Join(cacheDir, name))
		_ = os.Remove(pageCachePath(cacheDir, src))
	}
	pagesDir := filepath.Join(cacheDir, "pages")
	pageEntries, err := os.ReadDir(pagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range pageEntries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".html" {
			continue
		}
		src := name[:len(name)-len(".html")]
		if _, ok := alive[src]; ok {
			continue
		}
		_ = os.Remove(filepath.Join(pagesDir, name))
	}
	return nil
}
