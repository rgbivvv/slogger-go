package internal

import (
	"bytes"
	"encoding/binary"
	"log"
	"os"
	"path/filepath"
)

// StripMetadata removes EXIF/XMP/IPTC/comment data (camera details, GPS
// location, timestamps) from a JPEG or PNG in place without re-encoding, so
// image quality is unchanged. A JPEG's EXIF orientation is kept in a minimal
// EXIF block so the image still displays upright. Other formats, malformed
// files, and files with nothing to strip are left untouched.
func StripMetadata(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var out []byte
	var changed bool
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8}):
		out, changed = stripJPEG(data)
	case bytes.HasPrefix(data, pngSignature):
		out, changed = stripPNG(data)
	}
	if !changed {
		return nil
	}
	if err := replaceFile(path, out); err != nil {
		return err
	}
	log.Printf("stripped metadata from %s", path)
	return nil
}

// stripJPEG drops APP1 (EXIF, XMP), APP13 (Photoshop/IPTC) and COM segments.
// APP0 (JFIF), APP2 (ICC colour profile) and APP14 (Adobe colour transform)
// are kept because they affect how the image renders.
func stripJPEG(data []byte) ([]byte, bool) {
	out := []byte{0xFF, 0xD8}
	insertAt := len(out) // where a minimal EXIF block goes: after JFIF APP0
	orientation := 1
	changed := false
	for i := 2; ; {
		if i+2 > len(data) || data[i] != 0xFF {
			return data, false
		}
		marker := data[i+1]
		switch {
		case marker == 0xFF: // fill byte
			i++
			continue
		case marker == 0xDA: // start of scan: the rest is image data
			out = append(out, data[i:]...)
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7): // no length
			out = append(out, data[i:i+2]...)
			i += 2
			continue
		default:
			if i+4 > len(data) {
				return data, false
			}
			end := i + 2 + int(binary.BigEndian.Uint16(data[i+2:]))
			if end < i+4 || end > len(data) {
				return data, false
			}
			seg := data[i:end]
			switch marker {
			case 0xE1:
				if p := seg[4:]; len(p) > 14 && string(p[:6]) == "Exif\x00\x00" {
					orientation = exifOrientation(p[6:])
				}
				changed = true
			case 0xED, 0xFE:
				changed = true
			default:
				out = append(out, seg...)
				if marker == 0xE0 && insertAt == 2 {
					insertAt = len(out)
				}
			}
			i = end
			continue
		}
		break
	}
	if !changed {
		return data, false
	}
	if orientation != 1 {
		exif := orientationAPP1(orientation)
		out = append(out[:insertAt], append(exif, out[insertAt:]...)...)
	}
	return out, true
}

// orientationAPP1 builds an APP1 segment whose EXIF holds only the
// orientation tag.
func orientationAPP1(o int) []byte {
	tiff := []byte{
		'M', 'M', 0x00, 0x2A, 0x00, 0x00, 0x00, 0x08, // big-endian TIFF, IFD0 at 8
		0x00, 0x01, // one entry
		0x01, 0x12, 0x00, 0x03, 0x00, 0x00, 0x00, 0x01, // Orientation, SHORT, count 1
		0x00, byte(o), 0x00, 0x00, // value
		0x00, 0x00, 0x00, 0x00, // no next IFD
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	n := len(payload) + 2
	return append([]byte{0xFF, 0xE1, byte(n >> 8), byte(n)}, payload...)
}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// pngMetadataChunks are ancillary chunks that carry text or EXIF data and
// don't affect rendering.
var pngMetadataChunks = map[string]bool{"tEXt": true, "zTXt": true, "iTXt": true, "eXIf": true, "tIME": true}

func stripPNG(data []byte) ([]byte, bool) {
	out := append([]byte{}, pngSignature...)
	changed := false
	for i := len(pngSignature); i < len(data); {
		if i+12 > len(data) {
			return data, false
		}
		end := i + 12 + int(binary.BigEndian.Uint32(data[i:]))
		if end < i+12 || end > len(data) {
			return data, false
		}
		if pngMetadataChunks[string(data[i+4:i+8])] {
			changed = true
		} else {
			out = append(out, data[i:end]...)
		}
		i = end
	}
	return out, changed
}

// replaceFile atomically swaps path's contents, keeping its file mode.
func replaceFile(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".strip-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
