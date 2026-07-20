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

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
)

type Post struct {
	FnameSrc    string
	Fname       string
	Slug        string
	Title       string
	Epoch       int64
	Date        string
	FContent    string
	HTMLContent string
	Permalink   string
}

func ParsePosts(srcDir, assetsDir string, cfg *Config, policy *bluemonday.Policy) ([]Post, error) {
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

	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	if n > len(jobs) {
		n = len(jobs)
	}

	type result struct {
		post Post
		err  error
	}
	jobCh := make(chan string, len(jobs))
	resCh := make(chan result, len(jobs))

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			md := goldmark.New()
			for name := range jobCh {
				post, ok, err := parsePostFile(name, srcDir, assetsDir, cfg, policy, md)
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

	for _, name := range jobs {
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
	sort.Slice(posts, func(i, j int) bool { return posts[i].Epoch < posts[j].Epoch })
	return posts, nil
}

func parsePostFile(name, srcDir, assetsDir string, cfg *Config, policy *bluemonday.Policy, md goldmark.Markdown) (Post, bool, error) {
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
		log.Printf("Skipping %s: expected date_epoch[_title]", name)
		return Post{}, false, nil
	}
	date := parts[0]
	epoch, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		log.Printf("Skipping %s: epoch is not an integer", name)
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
	html := SanitizeHTML(buf.String(), policy)
	return Post{
		FnameSrc:    name,
		Fname:       slug + ".html",
		Slug:        slug,
		Title:       title,
		Epoch:       epoch,
		Date:        date,
		FContent:    content,
		HTMLContent: html,
		Permalink:   cfg.SiteURL + "/" + slug + ".html",
	}, true, nil
}

func WritePostPages(posts []Post, destDir string, cfg *Config, r *Renderer) (int, error) {
	written := 0
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
		header := fmt.Sprintf(
			`<table><tbody><tr><td>https://<a href="%s">%s</a>/%s</td></tr></tbody></table>`,
			cfg.SiteURL, cfg.SiteName, post.Fname,
		)
		back := `<p><a href="/">&#8604; Back to index</a></p>`
		if err := r.RenderWrite([]string{header, post.HTMLContent, back}, fpath); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}

// ParseFilenameEpoch is a small check helper for tests.
func ParseFilenameEpoch(stem string) (date string, epoch int64, ok bool) {
	parts := strings.Split(stem, "_")
	if len(parts) < 2 {
		return "", 0, false
	}
	epoch, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return parts[0], epoch, true
}
