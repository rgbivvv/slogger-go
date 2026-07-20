package internal

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParsePostsKeepsMediaHTML(t *testing.T) {
	src := t.TempDir()
	assets := t.TempDir()
	cache := t.TempDir()
	cfg := &Config{SiteURL: "https://example.com"}
	body := `<audio controls><source src="a.wav" type="audio/wav"></audio>
<video controls src="v.mp4"></video>`
	path := filepath.Join(src, "20260101_100_media.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	posts, err := parsePosts(src, assets, cfg, cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts", len(posts))
	}
	html := posts[0].HTMLContent
	for _, want := range []string{"<audio controls>", "<source src=\"a.wav\"", "<video controls src=\"v.mp4\">"} {
		if !strings.Contains(html, want) {
			t.Fatalf("HTML missing %q:\n%s", want, html)
		}
	}
	if strings.Contains(html, "raw HTML omitted") {
		t.Fatalf("raw HTML was stripped:\n%s", html)
	}
}

func TestParsePostsParallelSorted(t *testing.T) {
	src := t.TempDir()
	assets := t.TempDir()
	cache := t.TempDir()
	epochs := []int64{100, 50, 300, 200, 10, 400, 250, 75}
	for _, ep := range epochs {
		name := filepath.Join(src, "20260101_"+strconv.FormatInt(ep, 10)+"_post.md")
		if err := os.WriteFile(name, []byte("hello "+strconv.FormatInt(ep, 10)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &Config{SiteURL: "https://example.com"}
	posts, err := parsePosts(src, assets, cfg, cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != len(epochs) {
		t.Fatalf("got %d posts, want %d", len(posts), len(epochs))
	}
	for i := 1; i < len(posts); i++ {
		if posts[i].Epoch < posts[i-1].Epoch {
			t.Fatalf("not sorted: %d before %d", posts[i-1].Epoch, posts[i].Epoch)
		}
	}
}

func TestParsePostsCacheHitMiss(t *testing.T) {
	src := t.TempDir()
	assets := t.TempDir()
	cache := t.TempDir()
	cfg := &Config{SiteURL: "https://example.com"}
	srcName := "20260101_100_hello.md"
	path := filepath.Join(src, srcName)
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	posts, err := parsePosts(src, assets, cfg, cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts", len(posts))
	}
	cpath := cacheFilePath(cache, srcName)
	info1, err := os.Stat(cpath)
	if err != nil {
		t.Fatal("expected cache file after miss:", err)
	}

	posts2, err := parsePosts(src, assets, cfg, cache)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts2) != 1 || posts2[0].Epoch != 100 {
		t.Fatalf("warm parse: %+v", posts2)
	}
	info2, err := os.Stat(cpath)
	if err != nil {
		t.Fatal(err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Fatal("cache file rewritten on hit")
	}

	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	if _, err := parsePosts(src, assets, cfg, cache); err != nil {
		t.Fatal(err)
	}
	info3, err := os.Stat(cpath)
	if err != nil {
		t.Fatal(err)
	}
	if !info3.ModTime().After(info2.ModTime()) {
		t.Fatal("cache file not rewritten after mtime miss")
	}
}

func testRenderer(cfg *Config) *Renderer {
	return &Renderer{
		header: "<html><body>",
		footer: "</body></html>",
		vars: map[string]string{
			"title":          cfg.SiteName,
			"site_name":      cfg.SiteName,
			"copyright_name": cfg.CopyrightName,
		},
	}
}

func TestWritePostPagesCacheCopy(t *testing.T) {
	src := t.TempDir()
	assets := t.TempDir()
	cache := t.TempDir()
	dest := t.TempDir()
	cfg := &Config{SiteURL: "https://example.com", SiteName: "example.com"}
	r := testRenderer(cfg)

	srcName := "20260101_100_hello.md"
	srcName2 := "20260102_200_other.md"
	path := filepath.Join(src, srcName)
	path2 := filepath.Join(src, srcName2)
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte("other post"), 0o644); err != nil {
		t.Fatal(err)
	}

	posts, err := parsePosts(src, assets, cfg, cache)
	if err != nil {
		t.Fatal(err)
	}
	written, copied, err := WritePostPages(posts, dest, cache, cfg, r)
	if err != nil {
		t.Fatal(err)
	}
	if written != 2 || copied != 0 {
		t.Fatalf("first write: written=%d copied=%d", written, copied)
	}
	ppath := pageCachePath(cache, srcName)
	info1, err := os.Stat(ppath)
	if err != nil {
		t.Fatal("expected page cache file:", err)
	}

	posts2, err := parsePosts(src, assets, cfg, cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range posts2 {
		if !p.PageCached {
			t.Fatalf("expected PageCached for %s", p.FnameSrc)
		}
	}
	dest2 := t.TempDir()
	written, copied, err = WritePostPages(posts2, dest2, cache, cfg, r)
	if err != nil {
		t.Fatal(err)
	}
	if written != 0 || copied != 2 {
		t.Fatalf("second write: written=%d copied=%d", written, copied)
	}
	info2, err := os.Stat(ppath)
	if err != nil {
		t.Fatal(err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Fatal("page cache file rewritten on hit")
	}

	if err := os.WriteFile(path, []byte("hello world edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	posts3, err := parsePosts(src, assets, cfg, cache)
	if err != nil {
		t.Fatal(err)
	}
	dest3 := t.TempDir()
	written, copied, err = WritePostPages(posts3, dest3, cache, cfg, r)
	if err != nil {
		t.Fatal(err)
	}
	if written != 1 || copied != 1 {
		t.Fatalf("after edit: written=%d copied=%d, want 1 written 1 copied", written, copied)
	}

	if err := os.Remove(path2); err != nil {
		t.Fatal(err)
	}
	if _, err := parsePosts(src, assets, cfg, cache); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pageCachePath(cache, srcName2)); !os.IsNotExist(err) {
		t.Fatal("expected page cache pruned for deleted post")
	}
	if _, err := os.Stat(cacheFilePath(cache, srcName2)); !os.IsNotExist(err) {
		t.Fatal("expected json cache pruned for deleted post")
	}
}

func TestWritePostPagesCollisionUpdatesTitle(t *testing.T) {
	src := t.TempDir()
	assets := t.TempDir()
	cache := t.TempDir()
	dest := t.TempDir()
	cfg := &Config{SiteURL: "https://example.com", SiteName: "example.com"}
	r := testRenderer(cfg)

	// Leftover HTML from prior builds must not steal collision suffixes.
	for _, stale := range []string{"20260720_1.html", "20260720_2.html", "20260720_5.html"} {
		if err := os.WriteFile(filepath.Join(dest, stale), []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"20260720_100.md", "20260720_200.md", "20260720_300.md"} {
		if err := os.WriteFile(filepath.Join(src, name), []byte("body"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	posts, err := parsePosts(src, assets, cfg, cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := WritePostPages(posts, dest, cache, cfg, r); err != nil {
		t.Fatal(err)
	}
	want := []string{"20260720", "20260720_1", "20260720_2"}
	for i, slug := range want {
		if posts[i].Title != slug || posts[i].Slug != slug {
			t.Fatalf("post %d: Title=%q Slug=%q want %q", i, posts[i].Title, posts[i].Slug, slug)
		}
	}
}
