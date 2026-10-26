package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// testWebP builds only the container header of a lossy ("VP8 "), lossless ("VP8L") or extended ("VP8X") file.
func testWebP(kind string, w, h int) []byte {
	var payload []byte
	switch kind {
	case "VP8 ":
		payload = []byte{0x10, 0x02, 0x00, 0x9d, 0x01, 0x2a, byte(w), byte(w >> 8), byte(h), byte(h >> 8)}
	case "VP8L":
		bits := uint32(w-1) | uint32(h-1)<<14
		payload = append([]byte{0x2f}, binary.LittleEndian.AppendUint32(nil, bits)...)
	case "VP8X":
		payload = []byte{0, 0, 0, 0, byte(w - 1), byte((w - 1) >> 8), byte((w - 1) >> 16), byte(h - 1), byte((h - 1) >> 8), byte((h - 1) >> 16)}
	}
	chunk := append([]byte(kind), binary.LittleEndian.AppendUint32(nil, uint32(len(payload)))...)
	chunk = append(chunk, payload...)
	body := append([]byte("WEBP"), chunk...)
	return append(append([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, uint32(len(body)))...), body...)
}

func assetReasonOf(t *testing.T, err error) string {
	t.Helper()
	var input *InputError
	if !errors.As(err, &input) {
		t.Fatalf("err = %v, want an *InputError", err)
	}
	return input.Fields["file"]
}

func TestDetectImageAcceptsTheThreeFormats(t *testing.T) {
	cases := map[string]struct {
		data []byte
		want string
	}{
		"png":                        {testPNG(t, 4, 4), "image/png"},
		"jpeg":                       {testJPEG(t, 4, 4), "image/jpeg"},
		"webp lossy":                 {testWebP("VP8 ", 640, 480), "image/webp"},
		"webp lossless":              {testWebP("VP8L", 640, 480), "image/webp"},
		"webp extended":              {testWebP("VP8X", 640, 480), "image/webp"},
		"png at the dimension limit": {testPNG(t, 8000, 1), "image/png"},
	}
	for name, c := range cases {
		got, err := detectImage(c.data)
		if err != nil || got != c.want {
			t.Errorf("%s: detectImage = %q, %v; want %q", name, got, err, c.want)
		}
	}
}

func TestDetectImageRefusesWhatIsNotAnAllowedImage(t *testing.T) {
	cases := map[string][]byte{
		"svg":              []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"><script>alert(1)</script></svg>`),
		"svg with xml tag": []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`),
		"html":             []byte(`<html><body>hi</body></html>`),
		"pdf":              []byte("%PDF-1.7\n1 0 obj\n"),
		"gif":              []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"plain text":       []byte("not an image at all"),
		"empty":            nil,
		"riff but wav":     append(append([]byte("RIFF"), 4, 0, 0, 0), []byte("WAVE")...),
		"png then svg":     append([]byte("<svg/>"), testPNG(t, 2, 2)...),
	}
	for name, data := range cases {
		if _, err := detectImage(data); assetReasonOf(t, err) != ReasonAssetType {
			t.Errorf("%s: err = %v, want reason %s", name, err, ReasonAssetType)
		}
	}
}

func TestDetectImageRefusesCorruptData(t *testing.T) {
	good := testPNG(t, 32, 32)
	flipped := append([]byte(nil), good...)
	flipped[len(flipped)/2] ^= 0xff
	cases := map[string][]byte{
		"truncated png":        good[:len(good)/2],
		"png with a bad chunk": flipped,
		"png signature only":   good[:8],
		"truncated jpeg":       testJPEG(t, 32, 32)[:40],
		"webp riff too short":  testWebP("VP8L", 64, 64)[:14],
		"webp bad vp8 start":   append(append([]byte(nil), testWebP("VP8 ", 64, 64)[:23]...), 0, 0, 0, 0, 0),
	}
	for name, data := range cases {
		if _, err := detectImage(data); assetReasonOf(t, err) != ReasonAssetCorrupt {
			t.Errorf("%s: err = %v, want reason %s", name, err, ReasonAssetCorrupt)
		}
	}
}

func TestDetectImageRefusesAbsurdDimensions(t *testing.T) {
	cases := map[string][]byte{
		"png wide":      testPNG(t, 8001, 1),
		"png tall":      testPNG(t, 1, 8001),
		"jpeg wide":     testJPEG(t, 8001, 1),
		"webp lossy":    testWebP("VP8 ", 9000, 10),
		"webp lossless": testWebP("VP8L", 10, 9000),
		"webp extended": testWebP("VP8X", 20000, 20000),
	}
	for name, data := range cases {
		if _, err := detectImage(data); assetReasonOf(t, err) != ReasonAssetDimensions {
			t.Errorf("%s: err = %v, want reason %s", name, err, ReasonAssetDimensions)
		}
	}
}
