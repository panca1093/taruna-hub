package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

// importSheetRows builds a file: the template header plus one row per value map.
func importSheetRows(values ...map[string]string) [][]string {
	rows := [][]string{append([]string{}, importColumns...)}
	for _, value := range values {
		row := make([]string, len(importColumns))
		for at, column := range importColumns {
			row[at] = value[column]
		}
		rows = append(rows, row)
	}
	return rows
}

// importPG is one valid pg row over the harness curriculum.
func importPG(overrides map[string]string) map[string]string {
	row := map[string]string{
		"type": "pg", "program": "Program Soal", "subject": "Matematika", "chapter": "Bilangan",
		"difficulty": "easy", "content": "Hasil dari 2 + 2 adalah ...", "explanation": "2 + 2 = 4.",
		"option_a": "3", "option_b": "4", "correct": "B",
	}
	for key, value := range overrides {
		row[key] = value
	}
	return row
}

func importCSV(rows [][]string) []byte {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.WriteAll(rows)
	writer.Flush()
	return buf.Bytes()
}

func importXLSX(t *testing.T, rows [][]string) []byte {
	t.Helper()
	file := excelize.NewFile()
	t.Cleanup(func() { _ = file.Close() })
	const sheet = "Sheet1"
	for at, row := range rows {
		for column, value := range row {
			if value == "" {
				continue
			}
			ref, err := excelize.CoordinatesToCellName(column+1, at+1)
			if err != nil {
				t.Fatalf("cell name: %v", err)
			}
			if err := file.SetCellValue(sheet, ref, value); err != nil {
				t.Fatalf("set %s: %v", ref, err)
			}
		}
	}
	buf, err := file.WriteToBuffer()
	if err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	return buf.Bytes()
}

const (
	docxContentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`</Types>`
	docxRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`</Relationships>`
)

// docxOf zips a document whose only content is one table; a cell's "\n" becomes two paragraphs.
func docxOf(t *testing.T, rows [][]string) []byte {
	t.Helper()
	var document strings.Builder
	document.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	document.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	document.WriteString(`<w:body><w:tbl>`)
	for _, row := range rows {
		document.WriteString("<w:tr>")
		for _, value := range row {
			document.WriteString("<w:tc>")
			for _, paragraph := range strings.Split(value, "\n") {
				document.WriteString("<w:p><w:r><w:t>")
				if err := xml.EscapeText(&document, []byte(paragraph)); err != nil {
					t.Fatalf("escape cell: %v", err)
				}
				document.WriteString("</w:t></w:r></w:p>")
			}
			document.WriteString("</w:tc>")
		}
		document.WriteString("</w:tr>")
	}
	document.WriteString(`</w:tbl><w:p/></w:body></w:document>`)

	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for _, entry := range []struct{ name, body string }{
		{"[Content_Types].xml", docxContentTypesXML},
		{"_rels/.rels", docxRelsXML},
		{"word/document.xml", document.String()},
	} {
		writer, err := archive.Create(entry.name)
		if err != nil {
			t.Fatalf("zip %s: %v", entry.name, err)
		}
		if _, err := writer.Write([]byte(entry.body)); err != nil {
			t.Fatalf("write %s: %v", entry.name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func plainZip(t *testing.T, name, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	writer, err := archive.Create(name)
	if err != nil {
		t.Fatalf("zip %s: %v", name, err)
	}
	if _, err := writer.Write([]byte(body)); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// trimRow drops the trailing empty cells so the three formats compare as the user sees them.
func trimRow(row []string) []string {
	end := len(row)
	for end > 0 && strings.TrimSpace(row[end-1]) == "" {
		end--
	}
	return row[:end]
}

func TestImportParsesAllThreeFormats(t *testing.T) {
	rows := importSheetRows(
		importPG(nil),
		importPG(map[string]string{
			"type": "pg_multi", "subject": "Fisika", "chapter": "Gerak", "difficulty": "medium",
			"content": "Manakah bilangan genap?", "option_a": "2", "option_b": "3", "option_c": "4",
			"correct": "A,C",
		}),
		importPG(map[string]string{
			"type": "isian", "chapter": "", "content": "Ibu kota Jawa Barat\nadalah ...",
			"explanation": "Bandung.", "option_a": "", "option_b": "", "correct": "Bandung",
			"case_sensitive": "no",
		}),
		importPG(map[string]string{
			"type": "benar_salah", "subject": "Fisika", "chapter": "Gerak", "difficulty": "hard",
			"content": "Bumi itu bulat.", "explanation": "", "option_a": "", "option_b": "",
			"correct": "benar",
		}),
	)

	formats := []struct {
		name string
		data []byte
		want string
	}{
		{"soal.xlsx", importXLSX(t, rows), "xlsx"},
		{"soal.csv", append([]byte("\xef\xbb\xbf"), importCSV(rows)...), "csv"},
		{"soal.docx", docxOf(t, rows), "docx"},
	}

	var first [][]string
	for _, format := range formats {
		sheet, err := parseImportFile(format.name, format.data)
		if err != nil {
			t.Fatalf("%s: %v", format.name, err)
		}
		if sheet.format != format.want {
			t.Errorf("%s: format = %q, want %q", format.name, sheet.format, format.want)
		}
		if len(sheet.rows) != 4 {
			t.Fatalf("%s: %d data rows, want 4", format.name, len(sheet.rows))
		}
		got := make([][]string, 0, len(sheet.rows))
		for _, row := range sheet.rows {
			got = append(got, trimRow(row))
		}
		if first == nil {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("%s: %d rows, want %d as xlsx", format.name, len(got), len(first))
		}
		for at := range first {
			if strings.Join(got[at], "\x00") != strings.Join(first[at], "\x00") {
				t.Errorf("%s row %d = %q, want %q as xlsx", format.name, at+1, got[at], first[at])
			}
		}
	}
	if len(first) != 4 || first[2][5] != "Ibu kota Jawa Barat\nadalah ..." {
		t.Errorf("paragraph text in a cell = %q, want the two lines joined with a newline", first[2][5])
	}
}

func TestImportRejectsWrongContent(t *testing.T) {
	valid := importCSV(importSheetRows(importPG(nil)))
	headerOnly := importCSV(importSheetRows())
	missingColumn := importCSV([][]string{
		{"type", "program", "subject", "difficulty", "option_a", "option_b", "correct"},
		{"pg", "Program Soal", "Matematika", "easy", "3", "4", "B"},
	})
	manyRows := importSheetRows()
	for range maxImportRows + 1 {
		manyRows = append(manyRows, importSheetRows(importPG(nil))[1])
	}

	cases := []struct {
		name   string
		file   string
		data   []byte
		reason string
	}{
		{"xlsx holding csv bytes", "soal.xlsx", valid, ReasonImportFile},
		{"docx holding a plain zip", "soal.docx", plainZip(t, "hello.txt", "not a document"), ReasonImportFile},
		{"binary garbage as csv", "soal.csv", []byte{0x00, 0xff, 0xfe, 0x7f}, ReasonImportFile},
		{"csv with a NUL", "soal.csv", append(append([]byte{}, valid...), 0), ReasonImportFile},
		{"empty file", "soal.csv", nil, ReasonImportEmpty},
		{"header only", "soal.csv", headerOnly, ReasonImportEmpty},
		{"missing required column", "soal.csv", missingColumn, ReasonImportHeader},
		{"more than 500 rows", "soal.csv", importCSV(manyRows), ReasonImportRows},
		{"over 5 MB", "soal.csv", bytes.Repeat([]byte("a"), maxImportBytes+1), ReasonImportSize},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseImportFile(tc.file, tc.data)
			var file *ImportFileError
			if !errors.As(err, &file) {
				t.Fatalf("err = %v, want an ImportFileError", err)
			}
			if file.Reason != tc.reason {
				t.Errorf("reason = %q, want %q", file.Reason, tc.reason)
			}
		})
	}
}

func TestImportZipBombIsBounded(t *testing.T) {
	// A table the parser would accept, held in one cell that expands past the document cap.
	header := append([]string{}, importColumns...)
	huge := importPG(map[string]string{"content": strings.Repeat("a", maxDOCXXML+1024)})
	values := make([]string, len(importColumns))
	for at, column := range importColumns {
		values[at] = huge[column]
	}
	rows := [][]string{header, values}

	docxBomb := docxOf(t, rows)
	if len(docxBomb) > maxImportBytes {
		t.Fatalf("fixture is %d bytes, want a compressed bomb under the upload cap", len(docxBomb))
	}
	var failure *ImportFileError
	if _, err := parseImportFile("bomb.docx", docxBomb); !errors.As(err, &failure) || failure.Reason != ReasonImportFile {
		t.Fatalf("a document whose XML expands past the cap: err = %v, want a file rejection", err)
	}

	// The same trick for a workbook: real parts plus filler that pushes the archive past the budget.
	xlsxBomb := inflateZip(t, importXLSX(t, rows), maxXLSXUnzip, 1<<20)
	if len(xlsxBomb) > maxImportBytes {
		t.Fatalf("workbook bomb is %d bytes, want it under the upload cap", len(xlsxBomb))
	}
	if _, err := parseImportFile("bomb.xlsx", xlsxBomb); !errors.As(err, &failure) || failure.Reason != ReasonImportFile {
		t.Fatalf("a workbook whose archive expands past the cap: err = %v, want a file rejection", err)
	}
}

// inflateZip copies a workbook and adds filler parts until its uncompressed size passes limit.
func inflateZip(t *testing.T, data []byte, limit, chunk int) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("read the workbook: %v", err)
	}
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, entry := range reader.File {
		target, err := writer.Create(entry.Name)
		if err != nil {
			t.Fatalf("copy %s: %v", entry.Name, err)
		}
		source, err := entry.Open()
		if err != nil {
			t.Fatalf("open %s: %v", entry.Name, err)
		}
		if _, err := io.Copy(target, source); err != nil {
			t.Fatalf("copy %s: %v", entry.Name, err)
		}
		_ = source.Close()
	}
	padding := bytes.Repeat([]byte{0}, chunk)
	for at := 0; at*chunk <= limit+chunk; at++ {
		target, err := writer.Create(fmt.Sprintf("xl/filler%d.bin", at))
		if err != nil {
			t.Fatalf("add filler: %v", err)
		}
		if _, err := target.Write(padding); err != nil {
			t.Fatalf("write filler: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close the inflated zip: %v", err)
	}
	return buf.Bytes()
}

func TestImportKeepsFormulasAsText(t *testing.T) {
	h := newQuestionHarness(t)
	tenant := h.a

	rows := importSheetRows(
		importPG(map[string]string{"content": "=1+1"}),
		importPG(map[string]string{"content": "+SUM(A1)", "option_a": "1", "option_b": "2", "correct": "A"}),
		importPG(map[string]string{"content": "@cmd", "option_a": "1", "option_b": "2", "correct": "A"}),
	)
	data := importXLSX(t, rows)

	commit, err := h.commit(t, tenant, "formula.xlsx", data, []int{1, 2, 3})
	if err != nil {
		t.Fatalf("commit formulas: %v", err)
	}
	if commit.Created != 3 {
		t.Fatalf("created = %d, want 3", commit.Created)
	}

	want := map[int]string{1: "=1+1", 2: "+SUM(A1)", 3: "@cmd"}
	for at, id := range commit.QuestionIDs {
		question, err := h.svc.Get(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), id)
		if err != nil {
			t.Fatalf("get imported question: %v", err)
		}
		if question.Content != want[at+1] {
			t.Errorf("row %d content = %q, want the literal %q", at+1, question.Content, want[at+1])
		}
	}
}

func TestImportMapsRowErrorsToColumns(t *testing.T) {
	h := newQuestionHarness(t)
	rows := importSheetRows(
		importPG(map[string]string{"program": "Program Lain"}),
		importPG(map[string]string{"subject": "Biologi"}),
		importPG(map[string]string{"chapter": "Aljabar"}),
		importPG(map[string]string{"type": "essay"}),
		importPG(map[string]string{"correct": "D"}),
		importPG(map[string]string{"option_b": " 3 "}),
		importPG(map[string]string{"content": "   "}),
		importPG(map[string]string{"content": "Lihat ![x](data:image/png;base64,AAAA)"}),
		importPG(map[string]string{"correct": "A,B"}),
	)
	preview := h.preview(t, h.a, "salah.csv", importCSV(rows))

	want := []string{"program", "subject", "chapter", "type", "correct", "option_b", "content", "content", "correct"}
	if preview.Total != len(want) || preview.Invalid != len(want) || preview.Valid != 0 {
		t.Fatalf("preview = %d rows (%d valid, %d invalid), want %d invalid", preview.Total, preview.Valid, preview.Invalid, len(want))
	}
	for at, row := range preview.Rows {
		if row.Row != at+1 {
			t.Errorf("row number = %d, want %d", row.Row, at+1)
		}
		if len(row.Errors) == 0 {
			t.Errorf("row %d has no error", row.Row)
			continue
		}
		named := false
		for _, failure := range row.Errors {
			if failure.Field == want[at] {
				named = true
			}
			if !isImportColumn(failure.Field) {
				t.Errorf("row %d names %q, which is not a spreadsheet column", row.Row, failure.Field)
			}
		}
		if !named {
			t.Errorf("row %d errors = %+v, want one on %q", row.Row, row.Errors, want[at])
		}
	}
}

func isImportColumn(name string) bool {
	for _, column := range importColumns {
		if column == name {
			return true
		}
	}
	return false
}

func TestImportHtmlIsStoredVerbatim(t *testing.T) {
	h := newQuestionHarness(t)
	content := `<script>alert("x")</script> & ![x](https://example.com/a.png)`
	rows := importSheetRows(importPG(map[string]string{"content": content}))
	commit, err := h.commit(t, h.a, "html.csv", importCSV(rows), []int{1})
	if err != nil {
		t.Fatalf("commit html: %v", err)
	}
	question, err := h.svc.Get(context.Background(), h.scope(h.a), h.principal(h.a, model.RoleAdmin), commit.QuestionIDs[0])
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if question.Content != content {
		t.Errorf("content = %q, want the verbatim %q", question.Content, content)
	}
}

func TestPreviewWritesNothing(t *testing.T) {
	h := newQuestionHarness(t)
	rows := importSheetRows(importPG(nil), importPG(map[string]string{"program": "Program Lain"}))
	data := importCSV(rows)

	preview := h.preview(t, h.a, "campur.csv", data)
	if preview.Total != 2 || preview.Valid != 1 || preview.Invalid != 1 {
		t.Fatalf("preview = %d rows (%d valid, %d invalid), want 1 of each", preview.Total, preview.Valid, preview.Invalid)
	}
	if count := importQuestionCount(t, h, h.a.ID); count != 0 {
		t.Errorf("questions after a preview = %d, want 0", count)
	}
	if versions := importVersionCount(t, h, h.a.ID); versions != 0 {
		t.Errorf("versions after a preview = %d, want 0", versions)
	}
	if _, err := h.commit(t, h.a, "campur.csv", data, []int{99}); err == nil {
		t.Error("committing row 99 succeeded, want a rejection")
	}
	if count := importQuestionCount(t, h, h.a.ID); count != 0 {
		t.Errorf("questions after a rejected commit = %d, want 0", count)
	}
}

func TestCommitCreatesExactlySelectedRows(t *testing.T) {
	h := newQuestionHarness(t)
	rows := importSheetRows(
		importPG(map[string]string{"content": "Satu"}),
		importPG(map[string]string{"content": "Dua"}),
		importPG(map[string]string{"content": "Tiga"}),
	)
	commit, err := h.commit(t, h.a, "pilih.csv", importCSV(rows), []int{1, 3})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if commit.Created != 2 || commit.AlreadyImported != 0 || len(commit.QuestionIDs) != 2 {
		t.Fatalf("commit = %+v, want 2 created", commit)
	}
	if count := importQuestionCount(t, h, h.a.ID); count != 2 {
		t.Errorf("questions = %d, want 2", count)
	}

	contents := map[string]bool{}
	for _, id := range commit.QuestionIDs {
		question, err := h.svc.Get(context.Background(), h.scope(h.a), h.principal(h.a, model.RoleAdmin), id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		contents[question.Content] = true
	}
	if !contents["Satu"] || !contents["Tiga"] || contents["Dua"] {
		t.Errorf("stored contents = %v, want rows 1 and 3 only", contents)
	}
}

func TestCommitRejectsInvalidOrUnknownSelectedRows(t *testing.T) {
	h := newQuestionHarness(t)
	rows := importSheetRows(
		importPG(map[string]string{"content": "Satu"}),
		importPG(map[string]string{"content": "Dua"}),
		importPG(map[string]string{"content": "", "explanation": ""}),
	)
	data := importCSV(rows)

	_, err := h.commit(t, h.a, "campur.csv", data, []int{1, 2, 3})
	var invalid *ImportRowsError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want an ImportRowsError", err)
	}
	if len(invalid.Rows) != 1 || invalid.Rows[0] != 3 {
		t.Errorf("invalid rows = %v, want [3]", invalid.Rows)
	}

	for _, selected := range [][]int{{99}, {0}, {1, 1}, {}} {
		if _, err := h.commit(t, h.a, "campur.csv", data, selected); err == nil {
			t.Errorf("selection %v succeeded, want a 422", selected)
		}
	}

	// Rows that are not selected are ignored, even an invalid one.
	if _, err := h.commit(t, h.a, "campur.csv", data, []int{1, 2}); err != nil {
		t.Fatalf("commit the valid rows: %v", err)
	}
	if count := importQuestionCount(t, h, h.a.ID); count != 2 {
		t.Errorf("questions = %d, want exactly the 2 selected", count)
	}
}

func TestCommitRetryCreatesNoDuplicates(t *testing.T) {
	h := newQuestionHarness(t)
	data := importCSV(importSheetRows(importPG(nil), importPG(map[string]string{"content": "Dua"})))

	first, err := h.commit(t, h.a, "ulang.csv", data, []int{1, 2})
	if err != nil {
		t.Fatalf("first commit: %v", err)
	}
	second, err := h.commit(t, h.a, "ulang.csv", data, []int{1, 2})
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if first.Created != 2 || second.Created != 0 || second.AlreadyImported != 2 || len(second.QuestionIDs) != 0 {
		t.Fatalf("first = %+v second = %+v, want 2 created then 2 already imported", first, second)
	}
	if count := importQuestionCount(t, h, h.a.ID); count != 2 {
		t.Errorf("questions after the retry = %d, want 2", count)
	}
	if versions := importVersionCount(t, h, h.a.ID); versions != 2 {
		t.Errorf("versions after the retry = %d, want 2", versions)
	}
}

func TestCommitSubsetThenRestCreatesEachOnce(t *testing.T) {
	h := newQuestionHarness(t)
	data := importCSV(importSheetRows(
		importPG(map[string]string{"content": "Satu"}),
		importPG(map[string]string{"content": "Dua"}),
		importPG(map[string]string{"content": "Tiga"}),
	))

	first, err := h.commit(t, h.a, "sebagian.csv", data, []int{2})
	if err != nil {
		t.Fatalf("first commit: %v", err)
	}
	rest, err := h.commit(t, h.a, "sebagian.csv", data, []int{1, 2, 3})
	if err != nil {
		t.Fatalf("second commit: %v", err)
	}
	if first.Created != 1 || rest.Created != 2 || rest.AlreadyImported != 1 {
		t.Fatalf("first = %+v rest = %+v, want 1 then 2 created and 1 already there", first, rest)
	}
	if count := importQuestionCount(t, h, h.a.ID); count != 3 {
		t.Errorf("questions = %d, want 3", count)
	}
}

func TestConcurrentCommitsOfTheSameFileCreateEachRowOnce(t *testing.T) {
	h := newQuestionHarness(t)
	data := importCSV(importSheetRows(
		importPG(map[string]string{"content": "Satu"}),
		importPG(map[string]string{"content": "Dua"}),
		importPG(map[string]string{"content": "Tiga"}),
	))

	const writers = 6
	start := make(chan struct{})
	results := make([]model.ImportCommit, writers)
	failures := make([]error, writers)
	var wg sync.WaitGroup
	for at := range writers {
		wg.Add(1)
		go func(at int) {
			defer wg.Done()
			<-start
			results[at], failures[at] = h.commit(t, h.a, "bersamaan.csv", data, []int{1, 2, 3})
		}(at)
	}
	close(start)
	wg.Wait()

	created, already := 0, 0
	for at, err := range failures {
		if err != nil {
			t.Fatalf("writer %d: %v", at, err)
		}
		created += results[at].Created
		already += results[at].AlreadyImported
	}
	if created != 3 || already != 3*(writers-1) {
		t.Errorf("created = %d already imported = %d, want 3 and %d", created, already, 3*(writers-1))
	}
	if count := importQuestionCount(t, h, h.a.ID); count != 3 {
		t.Errorf("questions = %d, want 3", count)
	}
}

func TestImportedQuestionsAreFullQuestions(t *testing.T) {
	h := newQuestionHarness(t)
	rows := importSheetRows(
		importPG(nil),
		importPG(map[string]string{
			"type": "pg_multi", "subject": "Fisika", "chapter": "Gerak", "difficulty": "medium",
			"content": "Manakah bilangan genap?", "option_a": "2", "option_b": "3", "option_c": "4",
			"correct": "A,C",
		}),
		importPG(map[string]string{
			"type": "isian", "chapter": "", "content": "Ibu kota Jawa Barat adalah ...",
			"option_a": "", "option_b": "", "correct": "bandung", "case_sensitive": "no",
		}),
		importPG(map[string]string{
			"type": "benar_salah", "subject": "Fisika", "chapter": "Gerak", "difficulty": "sulit",
			"content": "Bumi itu bulat.", "option_a": "", "option_b": "", "correct": "true",
		}),
	)
	commit, err := h.commit(t, h.a, "lengkap.csv", importCSV(rows), []int{1, 2, 3, 4})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if commit.Created != 4 {
		t.Fatalf("created = %d, want 4", commit.Created)
	}

	ctx := context.Background()
	stored := make([]model.Question, 0, len(commit.QuestionIDs))
	for _, id := range commit.QuestionIDs {
		question, err := h.svc.Get(ctx, h.scope(h.a), h.principal(h.a, model.RoleAdmin), id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		stored = append(stored, question)
	}

	pg := stored[0]
	if pg.Version != 1 || pg.VersionID == uuid.Nil || len(pg.Options) != 2 {
		t.Fatalf("pg = version %d with %d options, want version 1 with 2", pg.Version, len(pg.Options))
	}
	if !isUUID(pg.Options[0].ID) || pg.Correct[0] != pg.Options[1].ID {
		t.Errorf("pg options = %+v correct = %v, want real ids with B correct", pg.Options, pg.Correct)
	}
	if pg.Difficulty != model.DifficultyEasy || pg.ProgramID != h.program(h.a).ID || pg.SubjectID != h.subject(h.a).ID {
		t.Errorf("pg = %+v, want the resolved curriculum", pg)
	}
	if stored[1].Difficulty != model.DifficultyMedium || len(stored[1].Correct) != 2 {
		t.Errorf("pg_multi = %+v, want medium with two correct ids", stored[1])
	}
	if stored[2].Type != model.QuestionIsian || len(stored[2].Options) != 0 || stored[2].Correct[0] != "bandung" {
		t.Errorf("isian = %+v, want the accepted answer without options", stored[2])
	}
	if stored[3].Type != model.QuestionTrueFalse || stored[3].Correct[0] != "benar" || stored[3].Difficulty != model.DifficultyHard {
		t.Errorf("benar_salah = %+v, want benar and hard", stored[3])
	}

	// An imported question is a normal question: the editor's PUT works on it.
	updated, err := h.update(t, h.a, pg.ID, QuestionInput{
		ProgramID: pg.ProgramID, SubjectID: pg.SubjectID, ChapterID: pg.ChapterID,
		Type: pg.Type, Content: "Hasil dari 3 + 3 adalah ...", Difficulty: pg.Difficulty,
		BaseVersion: pg.Version,
		Options:     []QuestionOptionInput{{ID: pg.Options[0].ID, Label: "6"}, {ID: pg.Options[1].ID, Label: "7"}},
		Correct:     []string{pg.Options[0].ID},
	})
	if err != nil {
		t.Fatalf("edit an imported question: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("version after the edit = %d, want 2", updated.Version)
	}

	// And the list filters see them.
	list, total := h.list(t, h.a, QuestionFilter{Type: model.QuestionIsian})
	if total != 1 || len(list) != 1 || list[0].ID != stored[2].ID {
		t.Errorf("filter by isian = %d questions (total %d), want the imported isian", len(list), total)
	}
	list, total = h.list(t, h.a, QuestionFilter{SubjectID: &h.tree[h.a.ID].Subjects[1].Subject.ID})
	if total != 2 {
		t.Errorf("filter by Fisika = %d (total %d), want 2 imported questions", len(list), total)
	}
}

func TestImportIsTenantScoped(t *testing.T) {
	h := newQuestionHarness(t)
	if _, err := repository.NewCurriculumRepository(h.pool).CreateTree(
		context.Background(), h.b.ID, model.ProgramTree{
			Program: model.Program{Code: "QB", Name: "Program B"},
			Subjects: []model.SubjectTree{
				{Subject: model.Subject{Name: "Biologi", Position: 0}},
			},
		}); err != nil {
		t.Fatalf("create tenant B curriculum: %v", err)
	}

	data := importCSV(importSheetRows(importPG(map[string]string{"program": "Program B", "subject": "Biologi", "chapter": ""})))

	preview := h.preview(t, h.a, "b.csv", data)
	if preview.Valid != 0 || len(preview.Rows[0].Errors) == 0 || preview.Rows[0].Errors[0].Field != "program" {
		t.Fatalf("preview in tenant A = %+v, want the program named as unknown", preview.Rows)
	}
	if _, err := h.commit(t, h.a, "b.csv", data, []int{1}); err == nil {
		t.Error("tenant A committed a file naming tenant B's program")
	}

	mine, err := h.commit(t, h.b, "b.csv", data, []int{1})
	if err != nil {
		t.Fatalf("tenant B commit: %v", err)
	}
	if mine.Created != 1 {
		t.Fatalf("created in B = %d, want 1", mine.Created)
	}
	if count := importQuestionCount(t, h, h.a.ID); count != 0 {
		t.Errorf("tenant A questions = %d, want 0", count)
	}
	if count := importQuestionCount(t, h, h.b.ID); count != 1 {
		t.Errorf("tenant B questions = %d, want 1", count)
	}

	// The same file imported by the other tenant is a new set of questions, not a conflict.
	shared := importCSV(importSheetRows(importPG(nil)))
	if _, err := h.commit(t, h.a, "sama.csv", shared, []int{1}); err != nil {
		t.Fatalf("tenant A commit: %v", err)
	}
	if _, err := h.commit(t, h.b, "sama.csv", shared, []int{1}); err != nil {
		t.Fatalf("tenant B commit: %v", err)
	}
	if count := importQuestionCount(t, h, h.a.ID); count != 1 {
		t.Errorf("tenant A questions = %d, want its own 1", count)
	}
	if count := importQuestionCount(t, h, h.b.ID); count != 2 {
		t.Errorf("tenant B questions = %d, want 2", count)
	}
}

func (h questionHarness) preview(t *testing.T, tenant model.Tenant, name string, data []byte) model.ImportPreview {
	t.Helper()
	preview, err := h.svc.PreviewImport(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), name, data)
	if err != nil {
		t.Fatalf("preview %s: %v", name, err)
	}
	return preview
}

func (h questionHarness) commit(t *testing.T, tenant model.Tenant, name string, data []byte, selected []int) (model.ImportCommit, error) {
	t.Helper()
	return h.svc.CommitImport(context.Background(), h.scope(tenant), h.principal(tenant, model.RoleAdmin), name, data, selected)
}

func importQuestionCount(t *testing.T, h questionHarness, tenantID uuid.UUID) int {
	t.Helper()
	var count int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM questions WHERE tenant_id = $1`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count questions: %v", err)
	}
	return count
}

func importVersionCount(t *testing.T, h questionHarness, tenantID uuid.UUID) int {
	t.Helper()
	var count int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM question_versions WHERE tenant_id = $1`, tenantID).Scan(&count); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	return count
}

func TestImportRejectsWideSheets(t *testing.T) {
	rows := importSheetRows(importPG(nil))
	rows[1] = append(rows[1], make([]string, 70-len(rows[1]))...)
	rows[1][69] = "x"
	var failure *ImportFileError
	if _, err := parseImportFile("wide.xlsx", importXLSX(t, rows)); !errors.As(err, &failure) || failure.Reason != ReasonImportFile {
		t.Fatalf("a cell past column %d: err = %v, want a file rejection", maxImportColumns, err)
	}
}

func TestImportDocxMergedCellsKeepColumns(t *testing.T) {
	cell := func(span, text string) string {
		pr := ""
		if span != "" {
			pr = `<w:tcPr><w:gridSpan w:val="` + span + `"/></w:tcPr>`
		}
		return `<w:tc>` + pr + `<w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:tc>`
	}
	body := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:tbl>` +
		`<w:tr>` + cell("", "a") + cell("", "b") + cell("", "c") + cell("", "d") + `</w:tr>` +
		`<w:tr>` + cell("", "1") + cell("2", "2") + cell("", "4") + `</w:tr>` +
		`<w:tr><w:trPr><w:gridBefore w:val="2"/></w:trPr>` + cell("", "3") + cell("", "4") + `</w:tr>` +
		`<w:tr>` + cell("0", "x") + cell("", "y") + `</w:tr>` +
		`</w:tbl></w:body></w:document>`
	rows, err := readDOCX(plainZip(t, "word/document.xml", body))
	if err != nil {
		t.Fatalf("read merged docx: %v", err)
	}
	want := [][]string{{"a", "b", "c", "d"}, {"1", "2", "", "4"}, {"", "", "3", "4"}, {"x", "y"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %q, want %q", rows, want)
	}
}

func TestImportDocxKeepsBreaksAndTabs(t *testing.T) {
	body := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:tbl><w:tr><w:tc><w:p>` +
		`<w:r><w:t>2</w:t><w:br/><w:t>3</w:t><w:tab/><w:t>4</w:t></w:r></w:p></w:tc></w:tr></w:tbl></w:body></w:document>`
	rows, err := readDOCX(plainZip(t, "word/document.xml", body))
	if err != nil {
		t.Fatalf("read docx: %v", err)
	}
	if got := rows[0][0]; got != "2\n3\t4" {
		t.Fatalf("cell = %q, want %q", got, "2\n3\t4")
	}
}

func TestCSVTemplateNeutralisesFormulaCells(t *testing.T) {
	data, err := writeCSV([][]string{{"=HYPERLINK(1)", "+x", "-1", "@a", "ok"}})
	if err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if got, want := strings.TrimSpace(string(data)), "'=HYPERLINK(1),'+x,'-1,'@a,ok"; got != want {
		t.Fatalf("template row = %q, want %q", got, want)
	}
}

func TestImportKeepsRealFormulaCellsAsText(t *testing.T) {
	file, err := excelize.OpenReader(bytes.NewReader(importXLSX(t, importSheetRows(importPG(nil)))))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = file.Close() }()
	column := slices.Index(importColumns, "content") + 1
	ref, _ := excelize.CoordinatesToCellName(column, 2)
	if err := file.SetCellFormula("Sheet1", ref, "1+1"); err != nil {
		t.Fatalf("set formula: %v", err)
	}
	buf, err := file.WriteToBuffer()
	if err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	sheet, err := parseImportFile("formula.xlsx", buf.Bytes())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := sheet.cell(sheet.rows[0], "content"); got != "=1+1" {
		t.Fatalf("formula cell = %q, want the literal %q", got, "=1+1")
	}
}
