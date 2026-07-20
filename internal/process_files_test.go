package internal

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
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
	epochs := []int64{100, 50, 300, 200, 10, 400, 250, 75}
	for _, ep := range epochs {
		name := filepath.Join(src, "20260101_"+strconv.FormatInt(ep, 10)+"_post.md")
		if err := os.WriteFile(name, []byte("hello "+strconv.FormatInt(ep, 10)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &Config{SiteURL: "https://example.com"}
	posts, err := ParsePosts(src, assets, cfg, SanitizePolicy())
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
