package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/webp"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// Per-element reasons of an upload preview; the transport turns each into the copy the dialog shows.
const (
	ElemDuplicate         = "duplicate"
	ElemDuplicateUpload   = "duplicate_in_upload"
	ElemSymbolEmoji       = "emoji_in_symbol"
	ElemSymbolMulti       = "symbol_multi"
	ElemSymbolInvalid     = "symbol_invalid"
	ElemImageType         = "image_type"
	ElemImageTooLarge     = "image_too_large"
	ElemImageCorrupt      = "image_corrupt"
	ElemImageDimensions   = "image_dimensions"
	ElemPathTraversal     = "path_traversal"
	ElemSymlink           = "symlink"
	ElemNestedArchive     = "nested_archive"
	ElemArchiveCorrupt    = "archive_corrupt"
	ElemArchiveTooLarge   = "archive_too_large"
	ElemArchiveTooMany    = "archive_too_many_files"
	ElemArchiveExtractBig = "archive_extracted_too_large"
	ElemNotInCatalog      = "not_in_catalog"
	ElemPackageFull       = "package_full"
	ElemUploadTooLarge    = "upload_too_large"
)

// D09 limits. The 5MB image ceiling is MaxAssetBytes; a ZIP is read from memory only after these checks.
const (
	maxZipBytes         = 10 << 20
	maxZipFiles         = 200
	maxZipExtracted     = 50 << 20
	maxUploadFiles      = 200
	maxSymbolTokens     = 1000
	maxSymbolTextBytes  = maxImportBytes
	MaxPackageElements  = 500
	maxElementLabelRune = 200
)

// UploadedFile is one multipart part: the client's name is only a label and a format hint, never a path.
type UploadedFile struct {
	Name string
	Data []byte
}

// ElementInput is everything one add-elements call may carry; the package type decides which parts count.
type ElementInput struct {
	Text  string
	Files []UploadedFile
	IDs   []string
}

// uploadBudget is shared by every file and archive of one request, so the per-archive limits cannot be
// multiplied by sending many archives: declared and inflated bytes stay within the 50MB D09 ceiling in total.
type uploadBudget struct {
	declared int64
	bytes    int64
	accepted int
}

func (b *uploadBudget) full() bool { return b.accepted >= MaxPackageElements }

type candidate struct {
	model.ElementCheck
	value       string
	data        []byte
	contentType string
}

func rejected(label, kind, reason string) candidate {
	return candidate{ElementCheck: model.ElementCheck{Label: label, Kind: kind, Reason: reason}}
}

// buildCandidates turns one upload into per-element verdicts. It touches no package and no storage, so a
// preview can never change anything; only a document that is unusable as a whole returns an error.
func buildCandidates(ctx context.Context, kind string, in ElementInput) ([]candidate, error) {
	switch kind {
	case model.ItemSymbol:
		return symbolCandidates(in)
	case model.ItemImage:
		return imageCandidates(ctx, in)
	case model.ItemEmoji:
		return catalogCandidates(in, model.ItemEmoji, emojiName)
	case model.ItemShape:
		return catalogCandidates(in, model.ItemShape, shapeName)
	}
	return nil, &InputError{Fields: map[string]string{"type": ReasonPackageType}}
}

func kindMismatch(field string) error {
	return &InputError{Fields: map[string]string{field: ReasonElementsKind}}
}

func catalogCandidates(in ElementInput, kind string, lookup func(string) (string, bool)) ([]candidate, error) {
	if strings.TrimSpace(in.Text) != "" || len(in.Files) > 0 {
		return nil, kindMismatch("elements")
	}
	if len(in.IDs) == 0 {
		return nil, &InputError{Fields: map[string]string{"elements": ReasonElementsEmpty}}
	}
	out := make([]candidate, 0, len(in.IDs))
	for _, raw := range in.IDs {
		id := strings.ToLower(strings.TrimSpace(raw))
		name, ok := lookup(id)
		if !ok {
			out = append(out, rejected(raw, kind, ElemNotInCatalog))
			continue
		}
		out = append(out, candidate{
			ElementCheck: model.ElementCheck{Label: name, Kind: kind, Key: id, Accepted: true},
			value:        id,
		})
	}
	return out, nil
}

func symbolCandidates(in ElementInput) ([]candidate, error) {
	if len(in.IDs) > 0 || len(in.Files) > 1 {
		return nil, kindMismatch("elements")
	}
	if len(in.Text) > maxSymbolTextBytes {
		return nil, &ImportFileError{Reason: ReasonImportSize}
	}
	tokens := strings.Fields(in.Text)
	if len(in.Files) == 1 {
		fromFile, err := symbolFileTokens(in.Files[0])
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, fromFile...)
	}
	if len(tokens) == 0 {
		return nil, &InputError{Fields: map[string]string{"elements": ReasonElementsEmpty}}
	}
	if len(tokens) > maxSymbolTokens {
		return nil, &ImportFileError{Reason: ReasonImportRows}
	}
	out := make([]candidate, 0, len(tokens))
	for _, token := range tokens {
		out = append(out, symbolCandidate(token))
	}
	return out, nil
}

// symbolFileTokens reads a one-column CSV, TXT or XLSX. A leading "simbol" header is skipped.
func symbolFileTokens(file UploadedFile) ([]string, error) {
	switch {
	case len(file.Data) == 0:
		return nil, &ImportFileError{Reason: ReasonImportEmpty}
	case len(file.Data) > maxImportBytes:
		return nil, &ImportFileError{Reason: ReasonImportSize}
	}
	var rows [][]string
	var err error
	switch strings.ToLower(filepath.Ext(file.Name)) {
	case ".txt":
		data := bytes.TrimPrefix(file.Data, []byte("\xef\xbb\xbf"))
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return nil, &ImportFileError{Reason: ReasonImportFile}
		}
		return strings.Fields(string(data)), nil
	case ".csv":
		rows, err = readCSV(file.Data)
	case ".xlsx":
		if !bytes.HasPrefix(file.Data, zipMagic) {
			return nil, &ImportFileError{Reason: ReasonImportFile}
		}
		rows, err = readXLSX(file.Data)
	default:
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	if err != nil {
		return nil, err
	}
	var tokens []string
	for at, row := range rows {
		if len(row) == 0 {
			continue
		}
		if at == 0 && isSymbolHeader(row[0]) {
			continue
		}
		tokens = append(tokens, strings.Fields(row[0])...)
	}
	return tokens, nil
}

func isSymbolHeader(cell string) bool {
	switch strings.ToLower(strings.TrimSpace(cell)) {
	case "simbol", "symbol":
		return true
	}
	return false
}

func symbolCandidate(token string) candidate {
	switch {
	case containsEmoji(token):
		return rejected(token, model.ItemSymbol, ElemSymbolEmoji)
	case !utf8.ValidString(token):
		return rejected(strings.ToValidUTF8(token, "?"), model.ItemSymbol, ElemSymbolInvalid)
	case utf8.RuneCountInString(token) != 1:
		return rejected(token, model.ItemSymbol, ElemSymbolMulti)
	}
	r, _ := utf8.DecodeRuneInString(token)
	if unicode.IsControl(r) || unicode.IsSpace(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
		return rejected(token, model.ItemSymbol, ElemSymbolInvalid)
	}
	return candidate{
		ElementCheck: model.ElementCheck{Label: token, Kind: model.ItemSymbol, Key: token, Accepted: true},
		value:        token,
	}
}

// containsEmoji is deliberately wider than the catalog: any pictographic block, the emoji-presentation
// symbols of the BMP, and the joiners that build sequences. Text symbols such as arrows, card suits and
// geometric shapes stay legal.
func containsEmoji(s string) bool {
	for _, r := range s {
		if isEmojiRune(r) {
			return true
		}
	}
	return false
}

func isEmojiRune(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF, r >= 0xE0020 && r <= 0xE007F:
		return true
	case r == 0x200D, r == 0xFE0F, r == 0x20E3:
		return true
	case r == 0x231A || r == 0x231B, r >= 0x23E9 && r <= 0x23F3, r >= 0x23F8 && r <= 0x23FA:
		return true
	case r == 0x25FD || r == 0x25FE, r == 0x2614 || r == 0x2615, r >= 0x2648 && r <= 0x2653:
		return true
	case r == 0x267F, r == 0x2693, r == 0x26A1, r == 0x26AA || r == 0x26AB, r == 0x26BD || r == 0x26BE:
		return true
	case r == 0x26C4 || r == 0x26C5, r == 0x26CE, r == 0x26D4, r == 0x26EA, r == 0x26F2 || r == 0x26F3:
		return true
	case r == 0x26F5, r == 0x26FA, r == 0x26FD, r == 0x2705, r == 0x270A || r == 0x270B, r == 0x2728:
		return true
	case r == 0x274C, r == 0x274E, r >= 0x2753 && r <= 0x2755, r == 0x2757, r >= 0x2795 && r <= 0x2797:
		return true
	case r == 0x27B0, r == 0x27BF, r == 0x2B1B || r == 0x2B1C, r == 0x2B50, r == 0x2B55:
		return true
	}
	return false
}

func imageCandidates(ctx context.Context, in ElementInput) ([]candidate, error) {
	if strings.TrimSpace(in.Text) != "" || len(in.IDs) > 0 {
		return nil, kindMismatch("elements")
	}
	if len(in.Files) == 0 {
		return nil, &InputError{Fields: map[string]string{"elements": ReasonElementsEmpty}}
	}
	if len(in.Files) > maxUploadFiles {
		return nil, &InputError{Fields: map[string]string{"elements": ReasonUploadTooMany}}
	}
	var out []candidate
	budget := &uploadBudget{}
	for _, file := range in.Files {
		label := elementLabel(file.Name)
		if strings.EqualFold(filepath.Ext(file.Name), ".zip") {
			out = append(out, zipCandidates(ctx, label, file.Data, budget)...)
			continue
		}
		if budget.bytes+int64(len(file.Data)) > maxZipExtracted {
			out = append(out, rejected(label, model.ItemImage, ElemUploadTooLarge))
			continue
		}
		budget.bytes += int64(len(file.Data))
		out = append(out, imageCandidate(ctx, label, file.Data, budget))
	}
	return out, nil
}

// imageCandidate proves the bytes are a PNG, JPEG or WebP that decodes; SVG and everything else fail as a type.
func imageCandidate(ctx context.Context, label string, data []byte, budget *uploadBudget) candidate {
	if budget.full() {
		return rejected(label, model.ItemImage, ElemPackageFull)
	}
	if len(data) > MaxAssetBytes {
		return rejected(label, model.ItemImage, ElemImageTooLarge)
	}
	if isArchive(data) {
		return rejected(label, model.ItemImage, ElemNestedArchive)
	}
	contentType, err := detectImage(ctx, data)
	if err != nil {
		var input *InputError
		if errors.As(err, &input) {
			return rejected(label, model.ItemImage, imageReason(input.Fields["file"]))
		}
		return rejected(label, model.ItemImage, ElemImageCorrupt)
	}
	if contentType == "image/webp" {
		if err := decodeWebP(ctx, data); err != nil {
			return rejected(label, model.ItemImage, imageReason(err.Error()))
		}
	}
	budget.accepted++
	sum := sha256.Sum256(data)
	return candidate{
		ElementCheck: model.ElementCheck{Label: label, Kind: model.ItemImage, Key: hex.EncodeToString(sum[:]), Accepted: true},
		data:         data, contentType: contentType,
	}
}

func imageReason(assetReason string) string {
	switch assetReason {
	case ReasonAssetType:
		return ElemImageType
	case ReasonAssetDimensions:
		return ElemImageDimensions
	}
	return ElemImageCorrupt
}

var archiveExtensions = map[string]bool{
	".zip": true, ".jar": true, ".tar": true, ".gz": true, ".tgz": true, ".bz2": true,
	".xz": true, ".7z": true, ".rar": true, ".zst": true,
}

func isArchive(data []byte) bool {
	return bytes.HasPrefix(data, zipMagic) || bytes.HasPrefix(data, []byte{0x1f, 0x8b}) ||
		bytes.HasPrefix(data, []byte("7z\xbc\xaf\x27\x1c")) || bytes.HasPrefix(data, []byte("Rar!\x1a\x07")) ||
		bytes.HasPrefix(data, []byte("BZh")) || bytes.HasPrefix(data, []byte("\xfd7zXZ\x00"))
}

// zipCandidates reads an archive entry by entry under hard bounds. An archive that breaks a global bound is
// rejected as one element, with nothing of it kept; a bad entry rejects only itself.
func zipCandidates(ctx context.Context, archiveLabel string, data []byte, budget *uploadBudget) []candidate {
	reject := func(reason string) []candidate { return []candidate{rejected(archiveLabel, model.ItemImage, reason)} }
	if len(data) > maxZipBytes {
		return reject(ElemArchiveTooLarge)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil && !(errors.Is(err, zip.ErrInsecurePath) && reader != nil) {
		return reject(ElemArchiveCorrupt)
	}

	var declared uint64
	files := 0
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		files++
		declared += entry.UncompressedSize64
	}
	switch {
	case files > maxZipFiles:
		return reject(ElemArchiveTooMany)
	case declared > maxZipExtracted:
		return reject(ElemArchiveExtractBig)
	case budget.declared+int64(declared) > maxZipExtracted || budget.bytes >= maxZipExtracted:
		return reject(ElemUploadTooLarge)
	}
	budget.declared += int64(declared)

	var out []candidate
	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() || junkEntry(entry.Name) {
			continue
		}
		label := elementLabel(entry.Name)
		switch {
		case unsafeEntryName(entry.Name):
			out = append(out, rejected(label, model.ItemImage, ElemPathTraversal))
			continue
		case entry.Mode()&os.ModeSymlink != 0:
			out = append(out, rejected(label, model.ItemImage, ElemSymlink))
			continue
		case archiveExtensions[strings.ToLower(path.Ext(entry.Name))]:
			out = append(out, rejected(label, model.ItemImage, ElemNestedArchive))
			continue
		case entry.UncompressedSize64 > MaxAssetBytes:
			out = append(out, rejected(label, model.ItemImage, ElemImageTooLarge))
			continue
		}
		body, err := readEntry(entry)
		if err != nil {
			out = append(out, rejected(label, model.ItemImage, ElemArchiveCorrupt))
			continue
		}
		// The declared sizes are the archive's own claim, so the bytes actually inflated are counted too, across the whole request.
		budget.bytes += int64(len(body))
		if budget.bytes > maxZipExtracted {
			return reject(ElemUploadTooLarge)
		}
		out = append(out, imageCandidate(ctx, label, body, budget))
	}
	if len(out) == 0 {
		return reject(ElemArchiveCorrupt)
	}
	return out
}

// readEntry inflates at most one byte past the image ceiling, so a lying size header cannot cost more.
func readEntry(entry *zip.File) ([]byte, error) {
	rc, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(io.LimitReader(rc, MaxAssetBytes+1))
}

// junkEntry is what macOS adds to an archive; it is skipped without a verdict.
func junkEntry(name string) bool {
	base := path.Base(name)
	return strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(base, "._") || base == ".DS_Store" || base == "Thumbs.db"
}

func unsafeEntryName(name string) bool {
	if name == "" || strings.ContainsAny(name, "\x00\\") || strings.HasPrefix(name, "/") {
		return true
	}
	if len(name) >= 2 && name[1] == ':' && unicode.IsLetter(rune(name[0])) {
		return true
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

// elementLabel is the dashboard label: the base name only, valid UTF-8 and bounded.
func elementLabel(name string) string {
	name = strings.ToValidUTF8(name, "?")
	name = strings.ReplaceAll(name, "\\", "/")
	label := strings.TrimSpace(path.Base(name))
	if label == "." || label == "/" {
		label = ""
	}
	if runes := []rune(label); len(runes) > maxElementLabelRune {
		label = string(runes[:maxElementLabelRune])
	}
	return label
}

// classify marks what the package already holds and what repeats inside the upload, then enforces the
// pool ceiling. The first occurrence wins, so the verdict is the same on a retry.
func classify(cands []candidate, existing []model.ItemElement) []candidate {
	held := make(map[string]bool, len(existing))
	for _, e := range existing {
		held[e.Key] = true
	}
	seen := map[string]bool{}
	total := len(existing)
	for at := range cands {
		c := &cands[at]
		if !c.Accepted {
			continue
		}
		switch {
		case held[c.Key]:
			c.Accepted, c.Reason = false, ElemDuplicate
		case seen[c.Key]:
			c.Accepted, c.Reason = false, ElemDuplicateUpload
		case total >= MaxPackageElements:
			c.Accepted, c.Reason = false, ElemPackageFull
		default:
			seen[c.Key] = true
			total++
		}
	}
	return cands
}

// decodeWebP fully decodes the image under the shared decode slots; the stdlib check in detectImage only
// reads the container header, so a corrupt payload would otherwise pass. The error text is an asset reason.
func decodeWebP(ctx context.Context, data []byte) error {
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return errors.New(ReasonAssetCorrupt)
	}
	if cfg.Width > maxAssetDimension || cfg.Height > maxAssetDimension {
		return errors.New(ReasonAssetDimensions)
	}
	select {
	case decodeSlots <- struct{}{}:
		defer func() { <-decodeSlots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if _, err := webp.Decode(bytes.NewReader(data)); err != nil {
		return errors.New(ReasonAssetCorrupt)
	}
	return nil
}
