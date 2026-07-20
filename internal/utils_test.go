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

func TestWipeDirFilesOnlyKeepsDir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "public")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WipeDirFilesOnly(sub); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(sub); err != nil || !info.IsDir() {
		t.Fatal("expected directory to remain")
	}
	if _, err := os.Stat(filepath.Join(sub, "a.txt")); !os.IsNotExist(err) {
		t.Fatal("expected file to be removed")
	}
}
