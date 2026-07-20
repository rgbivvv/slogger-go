package internal

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	mdImage = regexp.MustCompile(`!\[.*?\]\((https?://[^)]+)\)`)
	srcAttr = regexp.MustCompile(`src=['"](https?://[^'"]+)['"]`)
)

var (
	client     = &http.Client{Timeout: 10 * time.Second}
	downloadMu sync.Mutex
)

func LocalizeRemoteAssets(text, destDir string) string {
	urls := map[string]struct{}{}
	for _, m := range mdImage.FindAllStringSubmatch(text, -1) {
		urls[m[1]] = struct{}{}
	}
	for _, m := range srcAttr.FindAllStringSubmatch(text, -1) {
		urls[m[1]] = struct{}{}
	}

	for raw := range urls {
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			log.Printf("skipping local reference: %s", raw)
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		ext := strings.ToLower(path.Ext(u.Path))
		subdir := "other"
		switch ext {
		case ".mp3", ".wav", ".ogg", ".flac":
			subdir = "audio"
		case ".mp4", ".mpeg", ".mov", ".avi":
			subdir = "video"
		case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
			subdir = "img"
		}
		fname := path.Base(u.Path)
		dir := filepath.Join(destDir, subdir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("failed to create dir: %v", err)
			continue
		}
		destPath := filepath.Join(dir, fname)
		fetched := func() bool {
			downloadMu.Lock()
			defer downloadMu.Unlock()
			if _, err := os.Stat(destPath); os.IsNotExist(err) {
				if err := download(raw, destPath); err != nil {
					log.Printf("failed to fetch %s: %v", raw, err)
					return false
				}
			}
			return true
		}()
		if !fetched {
			continue
		}
		text = strings.ReplaceAll(text, raw, destPath)
	}
	return text
}

func download(rawURL, dest string) error {
	log.Printf("downloading file %s to %s", rawURL, dest)
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "curl/8.2.1")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
