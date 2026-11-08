package internal

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// insertSegments puts extra JPEG segments right after SOI.
func insertSegments(jpg []byte, segs ...[]byte) []byte {
	out := append([]byte{}, jpg[:2]...)
	for _, s := range segs {
		out = append(out, s...)
	}
	return append(out, jpg[2:]...)
}

func segment(marker byte, payload string) []byte {
	n := len(payload) + 2
	return append([]byte{0xFF, marker, byte(n >> 8), byte(n)}, payload...)
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStripMetadataJPEG(t *testing.T) {
	const secret = "GPSLatitude-52.3676"
	jfif := segment(0xE0, "JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")
	icc := segment(0xE2, "ICC_PROFILE\x00fake")
	xmp := segment(0xE1, "http://ns.adobe.com/xap/1.0/\x00"+secret)
	comment := segment(0xFE, "shot by "+secret)
	iptc := segment(0xED, "Photoshop 3.0\x00"+secret)

	// jpegWithOrientation gives SOI + EXIF(orientation 6) + image data.
	src := insertSegments(jpegWithOrientation(t, 40, 20, 6), jfif, icc, xmp, comment, iptc)

	p := writeTemp(t, "a.jpg", src)
	if err := StripMetadata(p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if bytes.Contains(got, []byte(secret)) {
		t.Error("metadata survived stripping")
	}
	if !bytes.HasPrefix(got[2:], jfif) {
		t.Error("JFIF APP0 should stay directly after SOI")
	}
	if !bytes.Contains(got, icc) {
		t.Error("ICC profile should be kept")
	}
	if o := jpegOrientation(bytes.NewReader(got)); o != 6 {
		t.Errorf("orientation = %d want 6", o)
	}
	if bytes.Count(got, []byte("Exif\x00\x00")) != 1 {
		t.Error("want exactly one (minimal) EXIF block")
	}
	f, _ := os.Open(p)
	defer f.Close()
	if cfg, _, err := image.DecodeConfig(f); err != nil || cfg.Width != 40 {
		t.Errorf("stripped JPEG no longer decodes: %v", err)
	}
	if w, h, _ := localImageSize(p); w != 20 || h != 40 {
		t.Errorf("display size = %dx%d want 20x40", w, h)
	}

	// Second pass has nothing left to strip besides the minimal EXIF, which
	// it rebuilds identically.
	if err := StripMetadata(p); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(p); !bytes.Equal(again, got) {
		t.Error("stripping is not idempotent")
	}
}

func TestStripMetadataJPEGUprightDropsAllEXIF(t *testing.T) {
	p := writeTemp(t, "b.jpg", jpegWithOrientation(t, 10, 10, 1))
	if err := StripMetadata(p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if bytes.Contains(got, []byte("Exif")) {
		t.Error("upright image should have no EXIF left")
	}
	if info, _ := os.Stat(p); info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v want 0644", info.Mode().Perm())
	}
}

func pngChunk(typ, data string) []byte {
	b := make([]byte, 8, 12+len(data))
	binary.BigEndian.PutUint32(b, uint32(len(data)))
	copy(b[4:], typ)
	b = append(b, data...)
	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(b[4:]))
}

func TestStripMetadataPNG(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	enc := buf.Bytes()
	ihdrEnd := 8 + 25 // signature + IHDR chunk
	src := append(append(append([]byte{}, enc[:ihdrEnd]...),
		append(pngChunk("tEXt", "Author\x00secret"), pngChunk("eXIf", "MM\x00*secret")...)...),
		enc[ihdrEnd:]...)

	p := writeTemp(t, "c.png", src)
	if err := StripMetadata(p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, enc) {
		t.Error("stripped PNG should equal the original encoding")
	}
}

func TestStripMetadataLeavesOtherFilesAlone(t *testing.T) {
	for name, data := range map[string][]byte{
		"x.gif":       []byte("GIF89a..."),
		"trunc.jpg":   {0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x40, 'E'},
		"trunc.png":   append(append([]byte{}, pngSignature...), 0, 0, 0, 9),
		"nothing.jpg": {0xFF, 0xD8, 0xFF, 0xDA, 0x00, 0x02, 1, 2, 0xFF, 0xD9},
	} {
		p := writeTemp(t, name, data)
		if err := StripMetadata(p); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if got, _ := os.ReadFile(p); !bytes.Equal(got, data) {
			t.Errorf("%s was modified", name)
		}
	}
}
