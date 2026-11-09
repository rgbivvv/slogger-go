package internal

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// jpegWithOrientation encodes a w x h JPEG whose left column is red, tagged
// with the given EXIF orientation.
func jpegWithOrientation(t *testing.T, w, h, orientation int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{0, 0, 255, 255}
			if x < w/4 {
				c = color.RGBA{255, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x01\x01\x12\x00\x03\x00\x00\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00")
	tiff[19] = byte(orientation) // big-endian SHORT value
	seg := append([]byte("Exif\x00\x00"), tiff...)
	app1 := append([]byte{0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}, seg...)
	data := buf.Bytes()
	return append(append([]byte{0xFF, 0xD8}, app1...), data[2:]...)
}

func TestLightenMedia(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "a.png")
	writePNG(t, imgPath, 40, 30)

	cases := []struct{ in, want string }{
		{`<img src="` + imgPath + `" alt="x">`, `<img src="` + imgPath + `" alt="x" loading="lazy" decoding="async" width="40" height="30">`},
		{`<img src="https://x.org/a.png" />`, `<img src="https://x.org/a.png" loading="lazy" decoding="async" />`},
		{`<img loading="eager" width="10" src="missing.png">`, `<img loading="eager" width="10" src="missing.png" decoding="async">`},
		{`<video controls src="v.mp4"></video>`, `<video controls src="v.mp4" preload="none"></video>`},
		{`<audio preload="auto" src="a.mp3">`, `<audio preload="auto" src="a.mp3">`},
		{`<p>no media</p>`, `<p>no media</p>`},
	}
	for _, c := range cases {
		if got := LightenMedia(c.in); got != c.want {
			t.Errorf("LightenMedia(%q)\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

func TestShrinkImage(t *testing.T) {
	dir := t.TempDir()

	small := filepath.Join(dir, "small.png")
	writePNG(t, small, 800, 600)
	before, _ := os.ReadFile(small)
	if err := ShrinkImage(small); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(small); !bytes.Equal(before, after) {
		t.Error("small image was rewritten")
	}

	wide := filepath.Join(dir, "wide.png")
	writePNG(t, wide, 2880, 1000)
	if err := ShrinkImage(wide); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(wide); err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("shrunk file mode = %v want 0644", info.Mode().Perm())
	}
	if w, h, _ := localImageSize(wide); w != maxImageWidth || h != 500 {
		t.Errorf("wide png = %dx%d want %dx500", w, h, maxImageWidth)
	}

	// 3000x2000 stored sideways (orientation 6 = rotate 90 CW): upright it
	// is 2000x3000, so it shrinks to 1440x2160 with the red stripe on top.
	rotated := filepath.Join(dir, "rot.jpg")
	if err := os.WriteFile(rotated, jpegWithOrientation(t, 3000, 2000, 6), 0o644); err != nil {
		t.Fatal(err)
	}
	if w, h, _ := localImageSize(rotated); w != 2000 || h != 3000 {
		t.Errorf("oriented size before shrink = %dx%d want 2000x3000", w, h)
	}
	if err := ShrinkImage(rotated); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(rotated)
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 1440 || b.Dy() != 2160 {
		t.Fatalf("rotated jpeg = %v want 1440x2160", b.Size())
	}
	if r, _, bl, _ := img.At(720, 50).RGBA(); r < bl {
		t.Error("top of rotated image should be red")
	}
	if r, _, bl, _ := img.At(720, 2100).RGBA(); r > bl {
		t.Error("bottom of rotated image should be blue")
	}
}

func TestLocalizeRemoteAssetsShrinksImages(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4000, 2000))); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dest := t.TempDir()
	out := LocalizeRemoteAssets("![x]("+srv.URL+"/big.png)", dest)
	local := filepath.Join(dest, "img", "big.png")
	if out != "![x]("+local+")" {
		t.Fatalf("got %q", out)
	}
	if w, h, ok := localImageSize(local); !ok || w != maxImageWidth || h != 720 {
		t.Errorf("mirrored image = %dx%d want %dx720", w, h, maxImageWidth)
	}
}
