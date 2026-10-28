package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

type zipEntry struct {
	name string
	data []byte
	mode os.FileMode
}

func buildZip(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		header := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			header.SetMode(e.mode)
		}
		part, err := w.CreateHeader(header)
		if err != nil {
			t.Fatalf("zip header %q: %v", e.name, err)
		}
		if _, err := part.Write(e.data); err != nil {
			t.Fatalf("zip write %q: %v", e.name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func byLabel(cands []candidate) map[string]candidate {
	out := map[string]candidate{}
	for _, c := range cands {
		out[c.Label] = c
	}
	return out
}

func wantRejected(t *testing.T, cands []candidate, label, reason string) {
	t.Helper()
	c, ok := byLabel(cands)[label]
	if !ok {
		t.Fatalf("no verdict for %q in %+v", label, cands)
	}
	if c.Accepted || c.Reason != reason {
		t.Errorf("%q: accepted=%v reason=%q, want rejected with %q", label, c.Accepted, c.Reason, reason)
	}
}

func wantAccepted(t *testing.T, cands []candidate, label string) {
	t.Helper()
	c, ok := byLabel(cands)[label]
	if !ok || !c.Accepted {
		t.Errorf("%q: %+v, want accepted", label, c)
	}
}

func symbolsOf(t *testing.T, text string) []candidate {
	t.Helper()
	cands, err := buildCandidates(context.Background(), model.ItemSymbol, ElementInput{Text: text})
	if err != nil {
		t.Fatalf("symbol candidates: %v", err)
	}
	return cands
}

func TestSymbolsAcceptTextSymbolsSeparatedBySpaceOrNewline(t *testing.T) {
	cands := symbolsOf(t, "♠ ♥\n★\t▲  ↔ A 7")
	if len(cands) != 7 {
		t.Fatalf("got %d candidates, want 7: %+v", len(cands), cands)
	}
	for _, c := range cands {
		if !c.Accepted || c.Key != c.Label || c.Kind != model.ItemSymbol {
			t.Errorf("%+v, want an accepted symbol keyed by itself", c)
		}
	}
}

func TestPastedEmojiFailsInASymbolPackageWithGuidance(t *testing.T) {
	cands := symbolsOf(t, "😀 ❤️ 👍🏽 ⭐ ✅ 🇮🇩 1️⃣ ♠")
	for _, label := range []string{"😀", "❤️", "👍🏽", "⭐", "✅", "🇮🇩", "1️⃣"} {
		wantRejected(t, cands, label, ElemSymbolEmoji)
	}
	wantAccepted(t, cands, "♠")
}

func TestSymbolRulesRejectMultiCharacterAndControlTokens(t *testing.T) {
	cands := symbolsOf(t, "ab ♠♥ ​ x")
	wantRejected(t, cands, "ab", ElemSymbolMulti)
	wantRejected(t, cands, "♠♥", ElemSymbolMulti)
	wantAccepted(t, cands, "x")
	if _, ok := byLabel(cands)["​"]; ok {
		wantRejected(t, cands, "​", ElemSymbolInvalid)
	}
}

func TestClassifyDedupsAgainstThePackageAndWithinTheUpload(t *testing.T) {
	existing := []model.ItemElement{{Key: "♠"}, {Key: "♥"}}
	cands := classify(symbolsOf(t, "♠ ★ ★ ♦ ♥"), existing)
	want := []struct {
		accepted bool
		reason   string
	}{{false, ElemDuplicate}, {true, ""}, {false, ElemDuplicateUpload}, {true, ""}, {false, ElemDuplicate}}
	for at, w := range want {
		if cands[at].Accepted != w.accepted || cands[at].Reason != w.reason {
			t.Errorf("candidate %d (%q): accepted=%v reason=%q, want %+v", at, cands[at].Label, cands[at].Accepted, cands[at].Reason, w)
		}
	}
}

func TestClassifyStopsAtThePackageCeiling(t *testing.T) {
	existing := make([]model.ItemElement, MaxPackageElements-1)
	for at := range existing {
		existing[at].Key = fmt.Sprintf("k%d", at)
	}
	cands := classify(symbolsOf(t, "a b c"), existing)
	if !cands[0].Accepted || cands[1].Reason != ElemPackageFull || cands[2].Reason != ElemPackageFull {
		t.Errorf("verdicts %+v, want one accepted and the rest package_full", cands)
	}
}

func TestSymbolFilesCsvTxtAndXlsx(t *testing.T) {
	csvData, err := writeCSV([][]string{{"simbol"}, {"♠"}, {"♥"}, {"😀"}})
	if err != nil {
		t.Fatal(err)
	}
	xlsxData, err := writeXLSX([][]string{{"simbol"}, {"★"}, {"▲"}})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		file     UploadedFile
		accepted []string
		emoji    []string
	}{
		"csv":  {UploadedFile{"s.csv", csvData}, []string{"♠", "♥"}, []string{"😀"}},
		"txt":  {UploadedFile{"s.txt", []byte("♠ ♥\n★\n")}, []string{"♠", "♥", "★"}, nil},
		"xlsx": {UploadedFile{"s.xlsx", xlsxData}, []string{"★", "▲"}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cands, err := buildCandidates(context.Background(), model.ItemSymbol, ElementInput{Files: []UploadedFile{tc.file}})
			if err != nil {
				t.Fatalf("candidates: %v", err)
			}
			for _, s := range tc.accepted {
				wantAccepted(t, cands, s)
			}
			for _, s := range tc.emoji {
				wantRejected(t, cands, s, ElemSymbolEmoji)
			}
			if _, ok := byLabel(cands)["simbol"]; ok {
				t.Errorf("the header was read as a symbol: %+v", cands)
			}
		})
	}
}

func TestSymbolFileRefusals(t *testing.T) {
	cases := map[string]UploadedFile{
		"wrong extension":   {Name: "s.png", Data: testPNG(t, 2, 2)},
		"xlsx without zip":  {Name: "s.xlsx", Data: []byte("not a zip")},
		"empty":             {Name: "s.txt", Data: nil},
		"binary in txt":     {Name: "s.txt", Data: []byte("a\x00b")},
		"oversize":          {Name: "s.txt", Data: bytes.Repeat([]byte("a "), maxImportBytes)},
		"too many symbols":  {Name: "s.txt", Data: []byte(strings.Repeat("a ", maxSymbolTokens+1))},
		"invalid utf8 text": {Name: "s.txt", Data: []byte{0xff, 0xfe, 0x20, 0x41}},
	}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := buildCandidates(context.Background(), model.ItemSymbol, ElementInput{Files: []UploadedFile{file}})
			var fileErr *ImportFileError
			if !errors.As(err, &fileErr) {
				t.Errorf("err = %v, want an *ImportFileError", err)
			}
		})
	}
}

func TestInputThatDoesNotBelongToThePackageTypeIsRefused(t *testing.T) {
	png := UploadedFile{Name: "a.png", Data: testPNG(t, 2, 2)}
	cases := map[string]struct {
		kind string
		in   ElementInput
	}{
		"symbol with ids":     {model.ItemSymbol, ElementInput{IDs: []string{"1f34e"}}},
		"symbol with 2 files": {model.ItemSymbol, ElementInput{Files: []UploadedFile{{Name: "a.txt"}, {Name: "b.txt"}}}},
		"image with text":     {model.ItemImage, ElementInput{Text: "♠", Files: []UploadedFile{png}}},
		"emoji with text":     {model.ItemEmoji, ElementInput{Text: "😀"}},
		"emoji with files":    {model.ItemEmoji, ElementInput{Files: []UploadedFile{png}}},
		"shape with ids none": {model.ItemShape, ElementInput{}},
		"image empty":         {model.ItemImage, ElementInput{}},
		"symbol empty":        {model.ItemSymbol, ElementInput{Text: "  \n "}},
		"unknown type":        {"mix", ElementInput{Text: "a"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := buildCandidates(context.Background(), tc.kind, tc.in)
			var input *InputError
			if !errors.As(err, &input) {
				t.Errorf("err = %v, want an *InputError", err)
			}
		})
	}
}

func TestCatalogPackagesAcceptOnlyCatalogIds(t *testing.T) {
	cands, err := buildCandidates(context.Background(), model.ItemEmoji, ElementInput{IDs: []string{"1F34E", "1f98a", "1f600-1f3fd", "😀", "nope"}})
	if err != nil {
		t.Fatal(err)
	}
	wantAccepted(t, cands, "Apel")
	wantAccepted(t, cands, "Rubah")
	for _, label := range []string{"1f600-1f3fd", "😀", "nope"} {
		wantRejected(t, cands, label, ElemNotInCatalog)
	}
	if cands[0].Key != "1f34e" {
		t.Errorf("key = %q, want the lowercase id", cands[0].Key)
	}

	shapes, err := buildCandidates(context.Background(), model.ItemShape, ElementInput{IDs: []string{"lingkaran", "bintang"}})
	if err != nil {
		t.Fatal(err)
	}
	wantAccepted(t, shapes, "Lingkaran")
	wantRejected(t, shapes, "bintang", ElemNotInCatalog)
}

func TestEmojiCatalogIsSingleCodepointAndFullyBundled(t *testing.T) {
	if len(emojiCatalog) == 0 {
		t.Fatal("empty emoji catalog")
	}
	seen := map[string]bool{}
	for _, e := range emojiCatalog {
		if seen[e.ID] {
			t.Errorf("duplicate catalog id %s", e.ID)
		}
		seen[e.ID] = true
		if strings.ContainsAny(e.ID, "-_ ") || e.ID != strings.ToLower(e.ID) {
			t.Errorf("%s is not a single lowercase codepoint (no skin tone, flag or ZWJ sequence allowed)", e.ID)
		}
		if _, err := os.Stat(filepath.Join("..", "..", "..", "web", "public", "emoji", e.ID+".svg")); err != nil {
			t.Errorf("catalog emoji %s has no bundled Twemoji asset: %v", e.ID, err)
		}
	}
	if _, err := os.Stat(filepath.Join("..", "..", "..", "web", "public", "emoji", "LICENSE-GRAPHICS.txt")); err != nil {
		t.Errorf("the Twemoji CC-BY license text is not bundled: %v", err)
	}
	for _, s := range symbolPalette {
		if containsEmoji(s) {
			t.Errorf("palette symbol %q would be refused by the symbol rules", s)
		}
	}
}

func TestImagesPlainFilesAreJudgedOneByOne(t *testing.T) {
	good := testPNG(t, 3, 3)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	cands, err := buildCandidates(context.Background(), model.ItemImage, ElementInput{Files: []UploadedFile{
		{Name: "a.png", Data: good},
		{Name: "b.jpg", Data: testJPEG(t, 4, 4)},
		{Name: "c.webp", Data: realWebP(t, "lossless")},
		{Name: "d.webp", Data: realWebP(t, "lossy")},
		{Name: "fake.webp", Data: testWebP("VP8L", 5, 5)},
		{Name: "logo.svg", Data: svg},
		{Name: "renamed.png", Data: svg},
		{Name: "broken.png", Data: good[:20]},
		{Name: "huge.png", Data: testPNG(t, 4097, 10)},
		{Name: "big.png", Data: append(append([]byte{}, good...), make([]byte, MaxAssetBytes)...)},
		{Name: "../../etc/passwd.png", Data: testPNG(t, 6, 6)},
		{Name: "inside.png", Data: buildZip(t, zipEntry{name: "x.png", data: good})},
	}})
	if err != nil {
		t.Fatal(err)
	}
	wantAccepted(t, cands, "a.png")
	wantAccepted(t, cands, "b.jpg")
	wantAccepted(t, cands, "c.webp")
	wantAccepted(t, cands, "d.webp")
	wantRejected(t, cands, "fake.webp", ElemImageCorrupt)
	wantRejected(t, cands, "logo.svg", ElemImageType)
	wantRejected(t, cands, "renamed.png", ElemImageType)
	wantRejected(t, cands, "broken.png", ElemImageCorrupt)
	wantRejected(t, cands, "huge.png", ElemImageDimensions)
	wantRejected(t, cands, "big.png", ElemImageTooLarge)
	wantRejected(t, cands, "inside.png", ElemNestedArchive)
	wantAccepted(t, cands, "passwd.png")
	if c := byLabel(cands)["a.png"]; len(c.Key) != 64 || c.contentType != "image/png" {
		t.Errorf("a.png key=%q type=%q, want a SHA-256 key and the detected type", c.Key, c.contentType)
	}
}

func TestImageDuplicatesAreByContentNotByName(t *testing.T) {
	same := testPNG(t, 3, 3)
	cands, err := buildCandidates(context.Background(), model.ItemImage, ElementInput{Files: []UploadedFile{
		{Name: "one.png", Data: same}, {Name: "copy-of-one.png", Data: same}, {Name: "two.png", Data: testPNG(t, 4, 4)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	cands = classify(cands, nil)
	wantAccepted(t, cands, "one.png")
	wantRejected(t, cands, "copy-of-one.png", ElemDuplicateUpload)
	wantAccepted(t, cands, "two.png")

	held := []model.ItemElement{{Key: byLabel(cands)["one.png"].Key}}
	again := classify(cands[:1:1], held)
	wantRejected(t, again, "one.png", ElemDuplicate)
}

func TestImageUploadFileCountIsBounded(t *testing.T) {
	files := make([]UploadedFile, maxUploadFiles+1)
	for at := range files {
		files[at] = UploadedFile{Name: "a.png"}
	}
	_, err := buildCandidates(context.Background(), model.ItemImage, ElementInput{Files: files})
	var input *InputError
	if !errors.As(err, &input) || input.Fields["elements"] != ReasonUploadTooMany {
		t.Errorf("err = %v, want upload_too_many", err)
	}
}

func TestZipEntriesAreJudgedAloneWithPerElementReasons(t *testing.T) {
	good, other := testPNG(t, 3, 3), testPNG(t, 4, 4)
	archive := buildZip(t,
		zipEntry{name: "fruits/good.png", data: good},
		zipEntry{name: "fruits/same-as-good.png", data: good},
		zipEntry{name: "other.jpg", data: testJPEG(t, 5, 5)},
		zipEntry{name: "../evil.png", data: other},
		zipEntry{name: "/abs.png", data: other},
		zipEntry{name: "back\\slash.png", data: other},
		zipEntry{name: "C:drive.png", data: other},
		zipEntry{name: "deep/../../up.png", data: other},
		zipEntry{name: "link.png", data: []byte("/etc/passwd"), mode: os.ModeSymlink | 0o777},
		zipEntry{name: "inner.zip", data: buildZip(t, zipEntry{name: "x.png", data: other})},
		zipEntry{name: "disguised.png", data: buildZip(t, zipEntry{name: "x.png", data: other})},
		zipEntry{name: "vector.svg", data: []byte("<svg/>")},
		zipEntry{name: "corrupt.png", data: good[:25]},
		zipEntry{name: "big.png", data: make([]byte, MaxAssetBytes+1)},
		zipEntry{name: "__MACOSX/._good.png", data: []byte("junk")},
		zipEntry{name: ".DS_Store", data: []byte("junk")},
		zipEntry{name: "folder/", data: nil},
	)
	cands, err := buildCandidates(context.Background(), model.ItemImage, ElementInput{Files: []UploadedFile{{Name: "pack.zip", Data: archive}}})
	if err != nil {
		t.Fatal(err)
	}
	cands = classify(cands, nil)

	wantAccepted(t, cands, "good.png")
	wantRejected(t, cands, "same-as-good.png", ElemDuplicateUpload)
	wantAccepted(t, cands, "other.jpg")
	wantRejected(t, cands, "evil.png", ElemPathTraversal)
	wantRejected(t, cands, "abs.png", ElemPathTraversal)
	wantRejected(t, cands, "slash.png", ElemPathTraversal)
	wantRejected(t, cands, "C:drive.png", ElemPathTraversal)
	wantRejected(t, cands, "up.png", ElemPathTraversal)
	wantRejected(t, cands, "link.png", ElemSymlink)
	wantRejected(t, cands, "inner.zip", ElemNestedArchive)
	wantRejected(t, cands, "disguised.png", ElemNestedArchive)
	wantRejected(t, cands, "vector.svg", ElemImageType)
	wantRejected(t, cands, "corrupt.png", ElemImageCorrupt)
	wantRejected(t, cands, "big.png", ElemImageTooLarge)
	for _, junk := range []string{"._good.png", ".DS_Store", "folder"} {
		if _, ok := byLabel(cands)[junk]; ok {
			t.Errorf("%q should be skipped without a verdict", junk)
		}
	}
	accepted := 0
	for _, c := range cands {
		if c.Accepted {
			accepted++
		}
	}
	if accepted != 2 {
		t.Errorf("accepted %d elements, want exactly the 2 good ones", accepted)
	}
}

func TestZipArchiveWideLimitsRejectTheArchiveAsOneElement(t *testing.T) {
	good := testPNG(t, 3, 3)

	many := make([]zipEntry, maxZipFiles+1)
	for at := range many {
		many[at] = zipEntry{name: fmt.Sprintf("f%03d.png", at), data: good}
	}
	// 11 entries of just under 5 MB: each is legal alone, together they pass 50 MB once extracted.
	bomb := make([]zipEntry, 11)
	for at := range bomb {
		bomb[at] = zipEntry{name: fmt.Sprintf("p%d.png", at), data: make([]byte, MaxAssetBytes-1)}
	}
	random := make([]byte, maxZipBytes+1)
	for at := range random {
		random[at] = byte(at*31 + at>>8)
	}
	truncated := buildZip(t, zipEntry{name: "a.png", data: good})
	truncated = truncated[:len(truncated)-30]

	cases := map[string]struct {
		data   []byte
		reason string
	}{
		"too many files":     {buildZip(t, many...), ElemArchiveTooMany},
		"extraction bomb":    {buildZip(t, bomb...), ElemArchiveExtractBig},
		"archive over 10 MB": {random, ElemArchiveTooLarge},
		"corrupt":            {[]byte("PK\x03\x04 this is not a zip at all"), ElemArchiveCorrupt},
		"truncated":          {truncated, ElemArchiveCorrupt},
		"empty archive":      {buildZip(t), ElemArchiveCorrupt},
		"only junk":          {buildZip(t, zipEntry{name: ".DS_Store", data: []byte("x")}), ElemArchiveCorrupt},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cands, err := buildCandidates(context.Background(), model.ItemImage, ElementInput{Files: []UploadedFile{{Name: "pack.zip", Data: tc.data}}})
			if err != nil {
				t.Fatal(err)
			}
			if len(cands) != 1 || cands[0].Accepted || cands[0].Reason != tc.reason || cands[0].Label != "pack.zip" {
				t.Errorf("verdicts %+v, want the archive alone rejected with %q", cands, tc.reason)
			}
		})
	}
}

func TestZipDeclaredSizeBombIsRefusedBeforeAnythingIsInflated(t *testing.T) {
	bomb := make([]zipEntry, 11)
	for at := range bomb {
		bomb[at] = zipEntry{name: fmt.Sprintf("p%d.png", at), data: make([]byte, MaxAssetBytes)}
	}
	data := buildZip(t, bomb...)
	if len(data) > maxZipBytes {
		t.Fatalf("fixture is %d bytes, it must stay under the 10 MB ZIP limit to reach the extraction check", len(data))
	}
	cands, err := buildCandidates(context.Background(), model.ItemImage, ElementInput{Files: []UploadedFile{{Name: "bomb.zip", Data: data}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].Reason != ElemArchiveExtractBig {
		t.Errorf("verdicts %+v, want the declared 55 MB rejected as archive_extracted_too_large", cands)
	}
}

func TestElementLabelKeepsOnlyABoundedBaseName(t *testing.T) {
	cases := map[string]string{
		"a/b/c.png":              "c.png",
		"..\\..\\win.png":        "win.png",
		"":                       "",
		"/":                      "",
		strings.Repeat("é", 300): strings.Repeat("é", maxElementLabelRune),
		"bad\xffname.png":        "bad?name.png",
	}
	for in, want := range cases {
		if got := elementLabel(in); got != want {
			t.Errorf("elementLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// realWebP are 1x1 files from a real encoder; testWebP only builds a header that the container check accepts.
func realWebP(t *testing.T, kind string) []byte {
	t.Helper()
	enc := map[string]string{
		"lossless": "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==",
		"lossy":    "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA",
	}[kind]
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRequestWideBudgetStopsManySmallArchives(t *testing.T) {
	// Each archive is far under every per-archive limit (one 4.9 MB image, a few KB zipped), so only a
	// request-wide budget keeps 200 of them from retaining about a gigabyte.
	files := make([]UploadedFile, maxUploadFiles)
	for at := range files {
		padded := append(testPNG(t, 3+at%40, 3+at/40), make([]byte, MaxAssetBytes-2000)...)
		files[at] = UploadedFile{Name: fmt.Sprintf("z%03d.zip", at), Data: buildZip(t, zipEntry{name: "i.png", data: padded})}
	}
	cands, err := buildCandidates(context.Background(), model.ItemImage, ElementInput{Files: files})
	if err != nil {
		t.Fatal(err)
	}
	var retained int64
	accepted, refused := 0, 0
	for _, c := range cands {
		retained += int64(len(c.data))
		switch {
		case c.Accepted:
			accepted++
		case c.Reason == ElemUploadTooLarge:
			refused++
		}
	}
	if retained > maxZipExtracted {
		t.Errorf("retained %d bytes, want at most %d for the whole request", retained, maxZipExtracted)
	}
	if accepted == 0 || refused < maxUploadFiles-accepted-2 {
		t.Errorf("accepted %d, refused %d of %d archives; want the first few kept and the rest upload_too_large", accepted, refused, len(files))
	}
}

func TestRequestWideBudgetCoversPlainFilesAndTheElementCeiling(t *testing.T) {
	big := append(testPNG(t, 3, 3), make([]byte, MaxAssetBytes-2000)...)
	files := make([]UploadedFile, 15)
	for at := range files {
		files[at] = UploadedFile{Name: fmt.Sprintf("p%d.png", at), Data: append(append([]byte{}, big...), byte(at))}
	}
	cands, err := buildCandidates(context.Background(), model.ItemImage, ElementInput{Files: files})
	if err != nil {
		t.Fatal(err)
	}
	var retained int64
	last := cands[len(cands)-1]
	for _, c := range cands {
		retained += int64(len(c.data))
	}
	if retained > maxZipExtracted || last.Reason != ElemUploadTooLarge {
		t.Errorf("retained %d, last verdict %+v; want the 50 MB request budget to refuse the tail", retained, last.ElementCheck)
	}

	small := make([]UploadedFile, 0, 152)
	for at := 0; at < 150; at++ {
		small = append(small, UploadedFile{Name: fmt.Sprintf("s%03d.png", at), Data: testPNG(t, 2+at%90, 2+at/90)})
	}
	var zips []zipEntry
	for at := 0; at < maxZipFiles; at++ {
		zips = append(zips, zipEntry{name: fmt.Sprintf("e%03d.png", at), data: testPNG(t, 2+at%90, 100+at/90)})
	}
	files = append(small, UploadedFile{Name: "a.zip", Data: buildZip(t, zips...)}, UploadedFile{Name: "b.zip", Data: buildZip(t, zips...)})
	cands, err = buildCandidates(context.Background(), model.ItemImage, ElementInput{Files: files})
	if err != nil {
		t.Fatal(err)
	}
	accepted := 0
	for _, c := range cands {
		if c.Accepted {
			accepted++
		}
	}
	if accepted != MaxPackageElements || cands[len(cands)-1].Reason != ElemPackageFull {
		t.Errorf("%d images accepted, last verdict %+v; want exactly the %d-element ceiling and package_full after it", accepted, cands[len(cands)-1].ElementCheck, MaxPackageElements)
	}
}

func TestSymbolTextIsCappedBeforeItIsSplit(t *testing.T) {
	_, err := buildCandidates(context.Background(), model.ItemSymbol, ElementInput{Text: strings.Repeat("a ", maxSymbolTextBytes)})
	var fileErr *ImportFileError
	if !errors.As(err, &fileErr) || fileErr.Reason != ReasonImportSize {
		t.Errorf("err = %v, want import_size", err)
	}
}
