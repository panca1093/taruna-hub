package service

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

const (
	maxImportBytes   = 5 << 20
	maxImportRows    = 500
	maxImportColumns = 64
	maxXLSXUnzip     = 50 << 20
	maxXLSXXML       = 20 << 20
	maxDOCXXML       = 20 << 20
)

// importColumns is the template contract: the header names, in the order the template writes them.
var importColumns = []string{
	"type", "program", "subject", "chapter", "difficulty", "content", "explanation",
	"option_a", "option_b", "option_c", "option_d", "option_e", "option_f", "option_g", "option_h",
	"correct", "case_sensitive",
}

var importRequiredColumns = []string{"type", "program", "subject", "difficulty", "content"}

var zipMagic = []byte("PK\x03\x04")

// importSheet is the parsed file: the header, the data rows, and where each column sits.
type importSheet struct {
	format  string
	columns map[string]int
	rows    [][]string
}

func (s importSheet) cell(row []string, name string) string {
	at, ok := s.columns[name]
	if !ok || at >= len(row) {
		return ""
	}
	return row[at]
}

// parseImportFile decides the format from the extension and then proves the content matches it.
func parseImportFile(name string, data []byte) (importSheet, error) {
	switch {
	case len(data) == 0:
		return importSheet{}, &ImportFileError{Reason: ReasonImportEmpty}
	case len(data) > maxImportBytes:
		return importSheet{}, &ImportFileError{Reason: ReasonImportSize}
	}

	var format string
	switch strings.ToLower(filepath.Ext(name)) {
	case ".xlsx", ".docx":
		if !bytes.HasPrefix(data, zipMagic) {
			return importSheet{}, &ImportFileError{Reason: ReasonImportFile}
		}
		format = strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	case ".csv":
		format = "csv"
	default:
		return importSheet{}, &ImportFileError{Reason: ReasonImportFile}
	}

	var rows [][]string
	var err error
	switch format {
	case "xlsx":
		rows, err = readXLSX(data)
	case "docx":
		rows, err = readDOCX(data)
	default:
		rows, err = readCSV(data)
	}
	if err != nil {
		return importSheet{}, err
	}
	if len(rows) == 0 {
		return importSheet{}, &ImportFileError{Reason: ReasonImportEmpty}
	}

	columns := make(map[string]int, len(rows[0]))
	for at, cell := range rows[0] {
		name := strings.ToLower(strings.TrimSpace(cell))
		if name == "" {
			continue
		}
		if _, ok := columns[name]; !ok {
			columns[name] = at
		}
	}
	for _, required := range importRequiredColumns {
		if _, ok := columns[required]; !ok {
			return importSheet{}, &ImportFileError{Reason: ReasonImportHeader}
		}
	}

	body := make([][]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if !blankRow(row) {
			body = append(body, row)
		}
	}
	if len(body) == 0 {
		return importSheet{}, &ImportFileError{Reason: ReasonImportEmpty}
	}
	if len(body) > maxImportRows {
		return importSheet{}, &ImportFileError{Reason: ReasonImportRows}
	}
	return importSheet{format: format, columns: columns, rows: body}, nil
}

func blankRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func readCSV(data []byte) ([][]string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	var rows [][]string
	kept := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return nil, &ImportFileError{Reason: ReasonImportFile}
		}
		rows = append(rows, row)
		if !blankRow(row) {
			kept++
		}
		if kept > maxImportRows+1 {
			return nil, &ImportFileError{Reason: ReasonImportRows}
		}
	}
}

func readXLSX(data []byte) ([][]string, error) {
	if !zipHasFile(data, "xl/workbook.xml") || !xlsxWithinColumns(data) {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	file, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit: maxXLSXUnzip, UnzipXMLSizeLimit: maxXLSXXML,
	})
	if err != nil {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	defer func() { _ = file.Close() }()
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	sheet := sheets[0]
	rows, err := file.GetRows(sheet)
	if err != nil {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	kept := 0
	for _, row := range rows {
		if !blankRow(row) {
			kept++
		}
	}
	if kept > maxImportRows+1 {
		return nil, &ImportFileError{Reason: ReasonImportRows}
	}
	// A formula cell stores no text of its own, so the formula replaces its cached result.
	for at, row := range rows {
		for column := range row {
			ref, err := excelize.CoordinatesToCellName(column+1, at+1)
			if err != nil {
				continue
			}
			formula, err := file.GetCellFormula(sheet, ref)
			if err == nil && formula != "" {
				rows[at][column] = "=" + formula
			}
		}
	}
	return rows, nil
}

// excelize pads every row out to its last column, so a lone cell far to the right would cost memory per row.
func xlsxWithinColumns(data []byte) bool {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, entry := range archive.File {
		if !strings.HasPrefix(entry.Name, "xl/worksheets/") || !strings.HasSuffix(entry.Name, ".xml") {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return false
		}
		within := cellsWithinColumns(io.LimitReader(reader, maxXLSXXML+1))
		_ = reader.Close()
		if !within {
			return false
		}
	}
	return true
}

func cellsWithinColumns(r io.Reader) bool {
	decoder := xml.NewDecoder(r)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return true
		}
		if err != nil {
			return false
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "c" {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "r" && cellColumn(attr.Value) > maxImportColumns {
				return false
			}
		}
	}
}

func cellColumn(ref string) int {
	column := 0
	for _, char := range strings.ToUpper(ref) {
		if char < 'A' || char > 'Z' {
			break
		}
		column = column*26 + int(char-'A') + 1
	}
	return column
}

func readDOCX(data []byte) ([][]string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	var document *zip.File
	for _, entry := range archive.File {
		if entry.Name == "word/document.xml" {
			document = entry
			break
		}
	}
	if document == nil {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	reader, err := document.Open()
	if err != nil {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	defer func() { _ = reader.Close() }()

	// The declared size is a claim, so the cap counts the bytes actually decompressed.
	body, err := io.ReadAll(io.LimitReader(reader, maxDOCXXML+1))
	if err != nil {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	if int64(len(body)) > maxDOCXXML {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	return parseDOCXTable(bytes.NewReader(body))
}

// parseDOCXTable reads the first table of a document; paragraphs in a cell join with a newline.
func parseDOCXTable(r io.Reader) ([][]string, error) {
	decoder := xml.NewDecoder(r)
	var (
		rows   [][]string
		row    []string
		cell   strings.Builder
		depth  int
		inRow  bool
		inCell bool
		inText bool
		span   int
	)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, &ImportFileError{Reason: ReasonImportFile}
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "tbl":
				depth++
			case "tr":
				if depth == 1 {
					row, inRow = []string{}, true
				}
			case "tc":
				if depth == 1 && inRow {
					inCell = true
					span = 1
					cell.Reset()
				}
			case "gridSpan":
				if depth == 1 && inCell {
					span = max(gridValue(element), 1)
				}
			case "gridBefore":
				if depth == 1 && inRow && !inCell {
					row = append(row, make([]string, gridValue(element))...)
				}
			case "br", "cr":
				if inCell {
					cell.WriteString("\n")
				}
			case "tab":
				if inCell {
					cell.WriteString("\t")
				}
			case "p":
				if inCell && cell.Len() > 0 {
					cell.WriteString("\n")
				}
			case "t":
				inText = inCell
			}
		case xml.CharData:
			if inText {
				cell.Write(element)
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "t":
				inText = false
			case "tc":
				if depth == 1 && inCell {
					row = append(row, cell.String())
					row = append(row, make([]string, span-1)...)
					inCell = false
				}
			case "tr":
				if depth == 1 && inRow {
					rows = append(rows, row)
					inRow = false
				}
			case "tbl":
				depth--
				if depth == 0 {
					return rows, nil
				}
			}
		}
	}
	if depth == 0 && len(rows) == 0 {
		return nil, &ImportFileError{Reason: ReasonImportFile}
	}
	return rows, nil
}

func gridValue(element xml.StartElement) int {
	for _, attr := range element.Attr {
		if attr.Name.Local == "val" {
			if n, err := strconv.Atoi(attr.Value); err == nil && n >= 0 {
				return min(n, maxImportColumns)
			}
		}
	}
	return 1
}

func zipHasFile(data []byte, name string) bool {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, entry := range archive.File {
		if entry.Name == name {
			return true
		}
	}
	return false
}

func writeCSV(rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	safe := make([][]string, len(rows))
	for at, row := range rows {
		safe[at] = make([]string, len(row))
		for column, value := range row {
			safe[at][column] = value
			if value != "" && strings.ContainsRune("=+-@", rune(value[0])) {
				safe[at][column] = "'" + value
			}
		}
	}
	if err := writer.WriteAll(safe); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeXLSX(rows [][]string) ([]byte, error) {
	const sheet = "Soal"
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	if err := file.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}
	for at, row := range rows {
		for column, value := range row {
			if value == "" {
				continue
			}
			ref, err := excelize.CoordinatesToCellName(column+1, at+1)
			if err != nil {
				return nil, err
			}
			if err := file.SetCellValue(sheet, ref, value); err != nil {
				return nil, err
			}
		}
	}
	buf, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const (
	docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`</Types>`

	docxRelationships = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`</Relationships>`
)

// writeDOCX builds the smallest document Word opens: one table, nothing else.
func writeDOCX(rows [][]string) ([]byte, error) {
	var document strings.Builder
	document.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	document.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	document.WriteString(`<w:body><w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/></w:tblPr>`)
	for _, row := range rows {
		document.WriteString("<w:tr>")
		for _, value := range row {
			document.WriteString(`<w:tc><w:p><w:r><w:t xml:space="preserve">`)
			if err := xml.EscapeText(&document, []byte(value)); err != nil {
				return nil, err
			}
			document.WriteString(`</w:t></w:r></w:p></w:tc>`)
		}
		document.WriteString("</w:tr>")
	}
	document.WriteString(`</w:tbl><w:p/><w:sectPr/></w:body></w:document>`)

	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for _, entry := range []struct{ name, body string }{
		{"[Content_Types].xml", docxContentTypes},
		{"_rels/.rels", docxRelationships},
		{"word/document.xml", document.String()},
	} {
		writer, err := archive.Create(entry.name)
		if err != nil {
			return nil, err
		}
		if _, err := writer.Write([]byte(entry.body)); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
