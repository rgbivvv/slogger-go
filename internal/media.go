package internal

import (
	"bufio"
	"encoding/binary"
	"html"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	_ "image/gif"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// maxImageWidth caps mirrored images at 2x the 45rem content column, which
// keeps them sharp on retina screens without shipping full camera originals.
const maxImageWidth = 1440

var (
	imgTagRe     = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	mediaTagRe   = regexp.MustCompile(`(?i)<(?:video|audio)\b[^>]*>`)
	tagSrcRe     = regexp.MustCompile(`(?i)\ssrc\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	loadingAttr  = regexp.MustCompile(`(?i)\sloading\s*=`)
	decodingAttr = regexp.MustCompile(`(?i)\sdecoding\s*=`)
	sizeAttr     = regexp.MustCompile(`(?i)\s(?:width|height)\s*=`)
	preloadAttr  = regexp.MustCompile(`(?i)\spreload\s*=`)
)

// LightenMedia defers media downloads in rendered post HTML: images get
// loading="lazy" decoding="async" (plus width/height for local files, so the
// layout doesn't jump), and video/audio get preload="none". Attributes the
// author already set are left alone.
func LightenMedia(htmlContent string) string {
	htmlContent = imgTagRe.ReplaceAllStringFunc(htmlContent, func(tag string) string {
		var add string
		if !loadingAttr.MatchString(tag) {
			add += ` loading="lazy"`
		}
		if !decodingAttr.MatchString(tag) {
			add += ` decoding="async"`
		}
		if !sizeAttr.MatchString(tag) {
			if m := tagSrcRe.FindStringSubmatch(tag); m != nil {
				if w, h, ok := localImageSize(html.UnescapeString(m[1] + m[2])); ok {
					add += ` width="` + strconv.Itoa(w) + `" height="` + strconv.Itoa(h) + `"`
				}
			}
		}
		return insertAttrs(tag, add)
	})
	return mediaTagRe.ReplaceAllStringFunc(htmlContent, func(tag string) string {
		if preloadAttr.MatchString(tag) {
			return tag
		}
		return insertAttrs(tag, ` preload="none"`)
	})
}

func insertAttrs(tag, attrs string) string {
	if attrs == "" {
		return tag
	}
	end := len(tag) - 1 // ">"
	if strings.HasSuffix(tag, "/>") {
		end--
		for end > 0 && tag[end-1] == ' ' {
			end--
		}
	}
	return tag[:end] + attrs + tag[end:]
}

// localImageSize returns the display size of an image referenced by a
// relative path on disk; remote and data: URLs are skipped.
func localImageSize(src string) (w, h int, ok bool) {
	if src == "" || strings.Contains(src, "://") || strings.HasPrefix(src, "//") || strings.HasPrefix(src, "data:") {
		return 0, 0, false
	}
	f, err := os.Open(src)
	if err != nil {
		if f, err = os.Open(strings.TrimLeft(src, "/")); err != nil {
			return 0, 0, false
		}
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, false
	}
	if format == "jpeg" {
		if _, err := f.Seek(0, io.SeekStart); err == nil && jpegOrientation(f) >= 5 {
			return cfg.Height, cfg.Width, true
		}
	}
	return cfg.Width, cfg.Height, true
}

// ShrinkImage downsizes a JPEG or PNG wider than maxImageWidth in place,
// applying any EXIF rotation first since re-encoding drops the EXIF block.
// Other formats and images already small enough are left untouched, so it is
// safe to call on every build.
func ShrinkImage(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil || (format != "jpeg" && format != "png") {
		return nil
	}
	orientation := 1
	if format == "jpeg" {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		orientation = jpegOrientation(f)
	}
	w := cfg.Width
	if orientation >= 5 {
		w = cfg.Height
	}
	if w <= maxImageWidth {
		return nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	src = applyOrientation(src, orientation)
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, maxImageWidth, b.Dy()*maxImageWidth/b.Dx()))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)

	info, err := f.Stat()
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".shrink-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	// CreateTemp makes 0600 files; keep the original's mode so the web
	// server can still read the image.
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	bw := bufio.NewWriter(tmp)
	if format == "jpeg" {
		err = jpeg.Encode(bw, dst, &jpeg.Options{Quality: 80})
	} else {
		err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(bw, dst)
	}
	if err == nil {
		err = bw.Flush()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	f.Close()
	return os.Rename(tmp.Name(), path)
}

// jpegOrientation reads the EXIF orientation tag (1-8) from a JPEG stream,
// returning 1 when it is absent or unreadable.
func jpegOrientation(r io.Reader) int {
	br := bufio.NewReader(r)
	var soi [2]byte
	if _, err := io.ReadFull(br, soi[:]); err != nil || soi != [2]byte{0xFF, 0xD8} {
		return 1
	}
	for {
		var hdr [4]byte
		if _, err := io.ReadFull(br, hdr[:]); err != nil || hdr[0] != 0xFF {
			return 1
		}
		marker, n := hdr[1], int(binary.BigEndian.Uint16(hdr[2:]))-2
		if marker == 0xDA || n < 0 { // start of scan: no more metadata
			return 1
		}
		seg := make([]byte, n)
		if _, err := io.ReadFull(br, seg); err != nil {
			return 1
		}
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return exifOrientation(seg[6:])
		}
	}
}

func exifOrientation(tiff []byte) int {
	var bo binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	ifd := int(bo.Uint32(tiff[4:8]))
	if ifd+2 > len(tiff) {
		return 1
	}
	count := int(bo.Uint16(tiff[ifd:]))
	for i := 0; i < count; i++ {
		e := ifd + 2 + i*12
		if e+12 > len(tiff) {
			return 1
		}
		if bo.Uint16(tiff[e:]) == 0x0112 {
			if o := int(bo.Uint16(tiff[e+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 1
		}
	}
	return 1
}

// applyOrientation returns src transformed so it displays upright for the
// given EXIF orientation.
func applyOrientation(src image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int
			switch o {
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			dst.Set(x, y, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}
