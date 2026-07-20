package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello_world"},
		{"20260126_1769475135 title", "20260126_1769475135_title"},
		{"  Foo---Bar  ", "foo_bar"},
		{"Café", "caf"}, // non-ASCII stripped
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestPruneStaleHTML(t *testing.T) {
	build := t.TempDir()
	public := filepath.Join(build, "public")
	if err := os.MkdirAll(public, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "old-slug.html", "current.html"} {
		if err := os.WriteFile(filepath.Join(build, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(public, "logo.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	keep := map[string]struct{}{
		"index.html":   {},
		"current.html": {},
	}
	if err := PruneStaleHTML(build, keep); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"index.html", "current.html"} {
		if _, err := os.Stat(filepath.Join(build, name)); err != nil {
			t.Fatalf("expected %s to remain: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(build, "old-slug.html")); !os.IsNotExist(err) {
		t.Fatal("expected old-slug.html to be removed")
	}
	if _, err := os.Stat(filepath.Join(public, "logo.png")); err != nil {
		t.Fatal("expected public/logo.png to remain:", err)
	}
}

func TestSyncTree(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "img", "a.png"), []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "keep.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SyncTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "img", "a.png")); err != nil || string(got) != "aaa" {
		t.Fatalf("first sync: got %q err %v", got, err)
	}

	// stale dest file should be pruned; unchanged files skipped
	if err := os.WriteFile(filepath.Join(dst, "stale.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(filepath.Join(dst, "keep.css"))
	if err != nil {
		t.Fatal(err)
	}
	if err := SyncTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "stale.txt")); !os.IsNotExist(err) {
		t.Fatal("expected stale.txt removed")
	}
	after, err := os.Stat(filepath.Join(dst, "keep.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("expected unchanged file to be skipped")
	}

	// source change should be copied
	if err := os.WriteFile(filepath.Join(src, "img", "a.png"), []byte("bbbb"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SyncTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "img", "a.png")); err != nil || string(got) != "bbbb" {
		t.Fatalf("after update: got %q err %v", got, err)
	}
}
