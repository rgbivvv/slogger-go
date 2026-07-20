package internal

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestParseFilenameEpoch(t *testing.T) {
	date, epoch, ok := ParseFilenameEpoch("20260126_1769475135_hello")
	if !ok || date != "20260126" || epoch != 1769475135 {
		t.Fatalf("got date=%q epoch=%d ok=%v", date, epoch, ok)
	}
	if _, _, ok := ParseFilenameEpoch("bad"); ok {
		t.Fatal("expected failure for bad stem")
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
	posts, err := parsePosts(src, assets, cfg, SanitizePolicy(), cache)
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

	posts, err := parsePosts(src, assets, cfg, SanitizePolicy(), cache)
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

	posts2, err := parsePosts(src, assets, cfg, SanitizePolicy(), cache)
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
	if _, err := parsePosts(src, assets, cfg, SanitizePolicy(), cache); err != nil {
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
