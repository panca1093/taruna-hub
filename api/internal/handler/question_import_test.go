package handler

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// importHeader is the template header, in the order the server writes it.
var importHeader = []string{
	"type", "program", "subject", "chapter", "difficulty", "content", "explanation",
	"option_a", "option_b", "option_c", "option_d", "option_e", "option_f", "option_g", "option_h",
	"correct", "case_sensitive",
}

// importCSVFile is one pg row, which the fixture's only program and subject accept.
func importCSVFile(contents ...string) []byte {
	rows := [][]string{importHeader}
	for _, content := range contents {
		row := make([]string, len(importHeader))
		values := map[string]string{
			"type": "pg", "program": "Sekolah Soal", "subject": "Matematika", "chapter": "Bilangan",
			"difficulty": "easy", "content": content, "explanation": "Karena begitu.",
			"option_a": "3", "option_b": "4", "correct": "B",
		}
		for at, column := range importHeader {
			row[at] = values[column]
		}
		rows = append(rows, row)
	}
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.WriteAll(rows); err != nil {
		panic(err)
	}
	writer.Flush()
	return buf.Bytes()
}

func importBody(t *testing.T, fileName string, data []byte, rows string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if data != nil {
		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			t.Fatalf("multipart file: %v", err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatalf("multipart write: %v", err)
		}
	}
	if rows != "" {
		if err := writer.WriteField("rows", rows); err != nil {
			t.Fatalf("multipart rows: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}
	return &buf, writer.FormDataContentType()
}

func postImport(t *testing.T, e *echo.Echo, host, path, token string, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set(echo.HeaderContentType, contentType)
	req.Host = host
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	return serve(e, req)
}

func previewFile(t *testing.T, e *echo.Echo, host, token, fileName string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := importBody(t, fileName, data, "")
	return postImport(t, e, host, "/api/v1/questions/import/preview", token, body, contentType)
}

func commitFile(t *testing.T, e *echo.Echo, host, token, fileName string, data []byte, rows string) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := importBody(t, fileName, data, rows)
	return postImport(t, e, host, "/api/v1/questions/import/commit", token, body, contentType)
}

func decodeImportPreview(t *testing.T, rec *httptest.ResponseRecorder) importPreviewResponse {
	t.Helper()
	var body importPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not an import preview: %v", rec.Body.String(), err)
	}
	return body
}

func decodeImportCommit(t *testing.T, rec *httptest.ResponseRecorder) importCommitResponse {
	t.Helper()
	var body importCommitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not an import commit: %v", rec.Body.String(), err)
	}
	return body
}

func TestImportRoles(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	operator := curriculumToken(t, f, tenant, model.RoleOperator)
	student := curriculumToken(t, f, tenant, model.RoleStudent)

	questionCurriculum(t, e, host, admin)
	data := importCSVFile("Hasil dari 2 + 2 adalah ...")

	type call func(token string) *httptest.ResponseRecorder
	routes := map[string]struct {
		call call
	}{
		"preview": {func(token string) *httptest.ResponseRecorder { return previewFile(t, e, host, token, "soal.csv", data) }},
		"commit": {func(token string) *httptest.ResponseRecorder {
			return commitFile(t, e, host, token, "soal.csv", data, "[1]")
		}},
		"template": {func(token string) *httptest.ResponseRecorder {
			return getWithToken(e, host, "/api/v1/questions/import/template?format=csv", token)
		}},
	}
	for name, route := range routes {
		if rec := route.call(""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s as anonymous: status = %d body = %s, want 401", name, rec.Code, rec.Body.String())
		}
		if rec := route.call(student); rec.Code != http.StatusForbidden {
			t.Errorf("%s as student: status = %d body = %s, want 403", name, rec.Code, rec.Body.String())
		}
	}

	if rec := routes["template"].call(operator); rec.Code != http.StatusOK {
		t.Errorf("template as operator: status = %d body = %s, want 200", rec.Code, rec.Body.String())
	}
	rec := routes["preview"].call(operator)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview as operator: status = %d body = %s, want 200", rec.Code, rec.Body.String())
	}
	if preview := decodeImportPreview(t, rec); preview.Total != 1 || preview.Valid != 1 {
		t.Errorf("preview = %+v, want one valid row", preview)
	}
	if rec := routes["commit"].call(operator); rec.Code != http.StatusOK {
		t.Fatalf("commit as operator: status = %d body = %s, want 200", rec.Code, rec.Body.String())
	}

	// The same file again is the same set of questions, not duplicates.
	body, contentType := importBody(t, "soal.csv", data, "[1]")
	if rec := postImport(t, e, host, "/api/v1/questions/import/commit", admin, body, contentType); rec.Code != http.StatusOK {
		t.Errorf("commit as admin: status = %d body = %s, want 200", rec.Code, rec.Body.String())
	} else if commit := decodeImportCommit(t, rec); commit.AlreadyImported != 1 || commit.Created != 0 {
		t.Errorf("admin retry = %+v, want 1 already imported", commit)
	}
}

func TestImportResponsesAreNoStore(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	questionCurriculum(t, e, host, admin)
	data := importCSVFile("Hasil dari 2 + 2 adalah ...")
	invalid := importCSVFile("")

	cases := []struct {
		name string
		rec  *httptest.ResponseRecorder
	}{
		{"template", getWithToken(e, host, "/api/v1/questions/import/template?format=xlsx", admin)},
		{"template without a token", getWithToken(e, host, "/api/v1/questions/import/template?format=xlsx", "")},
		{"preview", previewFile(t, e, host, admin, "soal.csv", data)},
		{"preview rejected", previewFile(t, e, host, admin, "soal.txt", data)},
		{"commit", commitFile(t, e, host, admin, "soal.csv", data, "[1]")},
		{"commit rejected", commitFile(t, e, host, admin, "soal.csv", data, "[]")},
		{"invalid row", commitFile(t, e, host, admin, "kosong.csv", invalid, "[1]")},
	}
	for _, tc := range cases {
		if got := tc.rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", tc.name, got)
		}
	}
}

func TestTemplatesRoundTripThroughTheParser(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	program := questionCurriculum(t, e, host, admin)

	wantTypes := map[string]string{
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"csv":  "text/csv; charset=utf-8",
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	}
	for format, contentType := range wantTypes {
		rec := getWithToken(e, host, "/api/v1/questions/import/template?format="+format, admin)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s template: status = %d body = %s, want 200", format, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get(echo.HeaderContentType); got != contentType {
			t.Errorf("%s template: content type = %q, want %q", format, got, contentType)
		}
		if got := rec.Header().Get(echo.HeaderContentDisposition); !strings.Contains(got, "template-soal."+format) {
			t.Errorf("%s template: Content-Disposition = %q, want the file name", format, got)
		}

		preview := previewFile(t, e, host, admin, "template."+format, rec.Body.Bytes())
		if preview.Code != http.StatusOK {
			t.Fatalf("%s template preview: status = %d body = %s", format, preview.Code, preview.Body.String())
		}
		body := decodeImportPreview(t, preview)
		if body.Total != 2 || body.Valid != 2 || body.Invalid != 0 {
			t.Errorf("%s template = %d rows (%d valid, %d invalid), want 2 valid", format, body.Total, body.Valid, body.Invalid)
		}
		if body.Rows[0].Valid && body.Rows[0].Preview != nil {
			if body.Rows[0].Preview.Type != "pg" || body.Rows[1].Preview.Type != "isian" {
				t.Errorf("%s template types = %q, %q; want pg then isian", format, body.Rows[0].Preview.Type, body.Rows[1].Preview.Type)
			}
			if body.Rows[0].Preview.Program != program.Name || body.Rows[0].Preview.Subject != program.Subjects[0].Name {
				t.Errorf("%s template names = %+v, want this tenant's program and subject", format, body.Rows[0].Preview)
			}
		}
	}
}

func TestImportRejectsBadFileAndRows(t *testing.T) {
	f := newAuthFixture(t, "localhost", false)
	e := f.echo(t)
	tenant := f.active[0]
	host := f.host(tenant.Slug)
	admin := curriculumToken(t, f, tenant, model.RoleAdmin)
	questionCurriculum(t, e, host, admin)
	data := importCSVFile("Hasil dari 2 + 2 adalah ...")

	missingFile, contentType := importBody(t, "", nil, "")
	wantError(t, "no file part",
		postImport(t, e, host, "/api/v1/questions/import/preview", admin, missingFile, contentType),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "file")

	missingRows, contentType := importBody(t, "soal.csv", data, "")
	wantError(t, "no rows",
		postImport(t, e, host, "/api/v1/questions/import/commit", admin, missingRows, contentType),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "rows")

	notJSON, contentType := importBody(t, "soal.csv", data, "1,2")
	wantError(t, "rows not JSON",
		postImport(t, e, host, "/api/v1/questions/import/commit", admin, notJSON, contentType),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "rows")

	unknownRow := commitFile(t, e, host, admin, "soal.csv", data, "[7]")
	wantError(t, "rows out of range", unknownRow, http.StatusUnprocessableEntity, httpx.CodeValidation, "rows")

	invalid := importCSVFile("")
	rec := commitFile(t, e, host, admin, "kosong.csv", invalid, "[1]")
	wantError(t, "selected invalid row", rec, http.StatusUnprocessableEntity, httpx.CodeValidation, "rows")
	if message := errorEnvelope(t, rec).Error.Fields["rows"]; !strings.Contains(message, "1") {
		t.Errorf("rows message = %q, want it to name row 1", message)
	}

	wrongFormat := previewFile(t, e, host, admin, "soal.xlsx", data)
	wantError(t, "csv bytes named xlsx", wrongFormat, http.StatusUnprocessableEntity, httpx.CodeValidation, "file")

	huge, contentType := importBody(t, "besar.csv", bytes.Repeat([]byte("a"), 7<<20), "")
	tooLarge := postImport(t, e, host, "/api/v1/questions/import/preview", admin, huge, contentType)
	if tooLarge.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("7 MB upload: status = %d body = %s, want 413", tooLarge.Code, tooLarge.Body.String())
	}
	if code := errorEnvelope(t, tooLarge).Error.Code; code != httpx.CodeTooLarge {
		t.Errorf("7 MB upload: error code = %q, want %q", code, httpx.CodeTooLarge)
	}
}
