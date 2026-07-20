package internal

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
)

type Post struct {
	FnameSrc    string
	Fname       string
	Slug        string
	Title       string
	Epoch       int64
	Date        string
	HTMLContent string
	Permalink   string
	PageCached  bool
}

func ParsePosts(srcDir, assetsDir string, cfg *Config) ([]Post, error) {
	return parsePosts(srcDir, assetsDir, cfg, ".slogger-cache")
}

func parsePosts(srcDir, assetsDir string, cfg *Config, cacheDir string) ([]Post, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}
	var jobs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		jobs = append(jobs, e.Name())
	}
	if len(jobs) == 0 {
		return nil, nil
	}

	alive := make(map[string]struct{}, len(jobs))
	var posts []Post
	var misses []string
	hits := 0

	for _, name := range jobs {
		alive[name] = struct{}{}
		path := filepath.Join(srcDir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() <= 1 {
			_ = os.Remove(path)
			delete(alive, name)
			continue
		}
		mtimeNano := info.ModTime().UnixNano()
		size := info.Size()
		if c, ok := loadCachedPost(cacheDir, name); ok && c.MtimeNano == mtimeNano && c.Size == size {
			posts = append(posts, c.toPost())
			hits++
			continue
		}
		misses = append(misses, name)
	}

	if len(misses) > 0 {
		parsed, err := parsePostMisses(misses, srcDir, assetsDir, cfg)
		if err != nil {
			return nil, err
		}
		for _, p := range parsed {
			path := filepath.Join(srcDir, p.FnameSrc)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			_ = saveCachedPost(cacheDir, p.FnameSrc, postToCached(p, info.ModTime().UnixNano(), info.Size()))
			posts = append(posts, p)
		}
	}

	if err := pruneCache(cacheDir, alive); err != nil {
		return nil, err
	}

	log.Printf("cache: %d hit, %d miss", hits, len(misses))
	sort.Slice(posts, func(i, j int) bool { return posts[i].Epoch < posts[j].Epoch })
	return posts, nil
}

func parsePostMisses(misses []string, srcDir, assetsDir string, cfg *Config) ([]Post, error) {
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	if n > len(misses) {
		n = len(misses)
	}

	type result struct {
		post Post
		err  error
	}
	jobCh := make(chan string, len(misses))
	resCh := make(chan result, len(misses))

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			md := NewMarkdown()
			for name := range jobCh {
				post, ok, err := parsePostFile(name, srcDir, assetsDir, cfg, md)
				if err != nil {
					resCh <- result{err: err}
					continue
				}
				if ok {
					resCh <- result{post: post}
				}
			}
		}()
	}

	for _, name := range misses {
		jobCh <- name
	}
	close(jobCh)
	wg.Wait()
	close(resCh)

	var posts []Post
	var firstErr error
	for r := range resCh {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		posts = append(posts, r.post)
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return posts, nil
}

func parsePostFile(name, srcDir, assetsDir string, cfg *Config, md goldmark.Markdown) (Post, bool, error) {
	path := filepath.Join(srcDir, name)
	info, err := os.Stat(path)
	if err != nil {
		return Post{}, false, nil
	}
	if info.Size() <= 1 {
		_ = os.Remove(path)
		return Post{}, false, nil
	}
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	parts := strings.Split(stem, "_")
	if len(parts) < 2 {
		log.Printf("skipping %s: expected date_epoch[_title]", name)
		return Post{}, false, nil
	}
	date := parts[0]
	epoch, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		log.Printf("skipping %s: epoch is not an integer", name)
		return Post{}, false, nil
	}
	titleParts := append([]string{date}, parts[2:]...)
	title := strings.Join(titleParts, " ")
	if title == "" {
		title = date
	}
	slug := Slugify(title)
	if slug == "" {
		slug = date
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return Post{}, false, err
	}
	content := LocalizeRemoteAssets(string(raw), assetsDir)
	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		return Post{}, false, err
	}
	return Post{
		FnameSrc:    name,
		Fname:       slug + ".html",
		Slug:        slug,
		Title:       title,
		Epoch:       epoch,
		Date:        date,
		HTMLContent: buf.String(),
		Permalink:   cfg.SiteURL + "/" + slug + ".html",
	}, true, nil
}

func WritePostPages(posts []Post, destDir, cacheDir string, cfg *Config, r *Renderer) (written, copied int, err error) {
	for i := range posts {
		post := &posts[i]
		base := post.Slug
		slug := base
		fpath := filepath.Join(destDir, slug+".html")
		suffix := 1
		for {
			if _, err := os.Stat(fpath); os.IsNotExist(err) {
				break
			}
			slug = fmt.Sprintf("%s_%d", base, suffix)
			fpath = filepath.Join(destDir, slug+".html")
			suffix++
		}
		if slug != base {
			post.Slug = slug
			post.Fname = slug + ".html"
			post.Permalink = cfg.SiteURL + "/" + slug + ".html"
		}

		if post.PageCached && cachedPageExists(cacheDir, post.FnameSrc) {
			if err := copyCachedPage(cacheDir, post.FnameSrc, fpath); err != nil {
				return written, copied, err
			}
			copied++
			continue
		}

		header := fmt.Sprintf(
			`<table><tbody><tr><td>https://<a href="%s">%s</a>/%s</td></tr></tbody></table>`,
			cfg.SiteURL, cfg.SiteName, post.Fname,
		)
		back := `<p><a href="/">&#8604; Back to index</a></p>`
		if err := r.RenderWrite([]string{header, post.HTMLContent, back}, fpath); err != nil {
			return written, copied, err
		}
		if err := saveCachedPage(cacheDir, post.FnameSrc, fpath); err != nil {
			return written, copied, err
		}
		written++
	}
	return written, copied, nil
}
