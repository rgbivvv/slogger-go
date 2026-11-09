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
	cases := []struct{ in, want string }{
		{`<img src="public/img/a.png" alt="x">`, `<img src="public/img/a.png" alt="x" loading="lazy" decoding="async">`},
		{`<img src="https://x.org/a.png" />`, `<img src="https://x.org/a.png" loading="lazy" decoding="async" />`},
		{`<img loading="eager" width="10" src="a.png">`, `<img loading="eager" width="10" src="a.png" decoding="async">`},
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

func TestLocalizeRemoteAssetsKeepsImagesUnchanged(t *testing.T) {
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
	if got, _ := os.ReadFile(local); !bytes.Equal(got, buf.Bytes()) {
		t.Error("mirrored image should be byte-for-byte identical")
	}
}
