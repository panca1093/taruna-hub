package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

type resultExportHarness struct {
	resultHarness
	exports *ExportService
	store   *memStore
	csv     *ResultExportRenderer
	xlsx    *ResultExportRenderer
}

func newResultExportHarness(t *testing.T) resultExportHarness {
	t.Helper()
	h := resultExportHarness{resultHarness: newResultHarness(t), store: &memStore{}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notes, err := NewNotificationService(repository.NewNotificationRepository(h.pool), noJobs{}, nil, bytes.Repeat([]byte{5}, SealKeyBytes), logger)
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	h.exports = NewExportService(h.pool, repository.NewExportRepository(h.pool), h.store, &countingExportJobs{}, notes, logger)
	h.csv = NewResultExportRenderer(h.pool, h.results, ExportFormatCSV, irtTestThreshold)
	h.xlsx = NewResultExportRenderer(h.pool, h.results, ExportFormatXLSX, irtTestThreshold)
	h.exports.Register(model.ExportTryoutResultsCSV, h.csv)
	h.exports.Register(model.ExportTryoutResultsXLSX, h.xlsx)
	t.Cleanup(func() {
		if _, err := h.pool.Exec(context.Background(), `DELETE FROM notification_logs WHERE tenant_id = $1 AND template = $2`, h.tenant().ID, TemplateExportReady); err != nil {
			t.Errorf("cleanup notices: %v", err)
		}
		if _, err := h.pool.Exec(context.Background(), `DELETE FROM exports WHERE tenant_id = $1`, h.tenant().ID); err != nil {
			t.Errorf("cleanup exports: %v", err)
		}
	})
	return h
}

func (h resultExportHarness) studentNamed(t *testing.T, name string, tr model.Tryout) model.Principal {
	t.Helper()
	user := walletUser(t, h.users, h.tenant().ID, model.RoleStudent, name)
	h.enroll(t, user, tr.ID)
	id := h.tenant().ID
	return model.Principal{UserID: user.ID, TenantID: &id, Name: name, Role: model.RoleStudent}
}

func (h resultExportHarness) setCompleted(t *testing.T, sessionID uuid.UUID, offset string) {
	t.Helper()
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE tryout_results SET completed_at = now() + $2::interval WHERE session_id = $1`, sessionID, offset); err != nil {
		t.Fatalf("set completed_at: %v", err)
	}
}

// request records an export through the real service and runs it once.
func (h resultExportHarness) request(t *testing.T, kind string, tryoutID, programID *uuid.UUID) model.Export {
	t.Helper()
	ctx := context.Background()
	admin := h.staff(model.RoleAdmin)
	params, err := h.rs.PrepareExport(ctx, h.scopeA(), admin, tryoutID, programID)
	if err != nil {
		t.Fatalf("prepare export: %v", err)
	}
	e, err := h.exports.Request(ctx, h.scopeA(), admin, kind, params)
	if err != nil {
		t.Fatalf("request export: %v", err)
	}
	if err := h.exports.Run(ctx, h.tenant().ID, e.ID, 1, ExportMaxAttempts); err != nil {
		t.Fatalf("run export: %v", err)
	}
	done, err := h.exports.Get(ctx, h.scopeA(), admin, e.ID)
	if err != nil || done.Status != model.ExportReady || done.ObjectKey == nil {
		t.Fatalf("export = %+v (err %v), want ready", done, err)
	}
	return done
}

func (h resultExportHarness) bytesOf(e model.Export) []byte { return h.store.objects[*e.ObjectKey] }

func csvRows(t *testing.T, data []byte) [][]string {
	t.Helper()
	if !bytes.HasPrefix(data, []byte("\xEF\xBB\xBF")) {
		t.Fatal("csv lacks the UTF-8 BOM")
	}
	if !bytes.Contains(data, []byte("\r\n")) {
		t.Error("csv lines are not CRLF terminated")
	}
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF")))).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	return rows
}

func xlsxFile(t *testing.T, data []byte) (*excelize.File, [][]string) {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	rows, err := f.GetRows(resultExportSheet)
	if err != nil {
		t.Fatalf("read xlsx rows: %v", err)
	}
	return f, rows
}

func TestResultExportCutOffExcludesLaterCompletionsAndFormatsAgree(t *testing.T) {
	h := newResultExportHarness(t)
	q1 := h.question(t)
	tr := h.tryoutOf(t, nil, q1)
	right, wrong := optID(t, q1, "4"), optID(t, q1, "3")

	names := []string{`=HYPERLINK("http://x")`, "+cmd", "@SUM(1)", "-2+3", "Ani, \"Si\" Rajin"}
	for i, name := range names {
		answer := right
		if i == 1 {
			answer = wrong
		}
		p := h.studentNamed(t, name, tr)
		sid := h.take(t, p, tr, map[uuid.UUID]string{q1.ID: answer})
		h.setCompleted(t, sid, "-1 hour")
	}
	late := h.studentNamed(t, "Terlambat", tr)
	lateSession := h.take(t, late, tr, map[uuid.UUID]string{q1.ID: right})
	h.setCompleted(t, lateSession, "1 hour")

	csvExport := h.request(t, model.ExportTryoutResultsCSV, &tr.ID, nil)
	xlsxExport := h.request(t, model.ExportTryoutResultsXLSX, &tr.ID, nil)
	if csvExport.FileName == nil || !strings.HasPrefix(*csvExport.FileName, "hasil-tryout-") || !strings.HasSuffix(*csvExport.FileName, ".csv") ||
		!strings.HasSuffix(*xlsxExport.FileName, ".xlsx") {
		t.Errorf("file names = %v / %v, want hasil-tryout-<date>.csv/.xlsx", csvExport.FileName, xlsxExport.FileName)
	}

	cRows := csvRows(t, h.bytesOf(csvExport))
	f, xRows := xlsxFile(t, h.bytesOf(xlsxExport))
	if len(cRows) != len(names)+1 {
		t.Fatalf("csv rows = %d, want header + %d (the later completion is excluded)", len(cRows), len(names))
	}
	if strings.Join(cRows[0], "|") != strings.Join(resultExportHeader, "|") {
		t.Errorf("csv header = %v", cRows[0])
	}
	if len(xRows) != len(cRows) {
		t.Fatalf("xlsx rows = %d, csv rows = %d, want the same rows", len(xRows), len(cRows))
	}
	for i := range cRows {
		if strings.Join(cRows[i], "|") != strings.Join(xRows[i], "|") {
			t.Errorf("row %d differs: csv %v, xlsx %v", i, cRows[i], xRows[i])
		}
	}
	for _, rows := range [][][]string{cRows, xRows} {
		seen := map[string]bool{}
		for _, row := range rows[1:] {
			seen[row[0]] = true
			if row[0] == "Terlambat" {
				t.Error("a result completed after the cut-off is in the report")
			}
			if row[6] == "" || row[9] == "" || !strings.HasSuffix(row[9], "+07:00") {
				t.Errorf("row %v misses rank or a Jakarta timestamp", row)
			}
		}
		for _, name := range []string{`'=HYPERLINK("http://x")`, "'+cmd", "'@SUM(1)", "'-2+3", `Ani, "Si" Rajin`} {
			if !seen[name] {
				t.Errorf("names %v lack the neutralized %q", seen, name)
			}
		}
	}

	// The rank is judged over the same cut-off set: 4 students at 100 share rank 1, the wrong answer ranks 5th.
	for _, row := range xRows[1:] {
		want := "1"
		if row[0] == "+cmd" || row[0] == "'+cmd" {
			want = "5"
		}
		if row[6] != want || row[7] != "5" {
			t.Errorf("%s: rank %s of %s, want %s of 5", row[0], row[6], row[7], want)
		}
	}

	for _, ref := range []string{"D2", "E2", "F2", "G2", "H2"} {
		typ, err := f.GetCellType(resultExportSheet, ref)
		// excelize reports a plain numeric cell (no t attribute) as CellTypeUnset.
		if err != nil || (typ != excelize.CellTypeNumber && typ != excelize.CellTypeUnset) {
			t.Errorf("cell %s type = %v (err %v), want numeric", ref, typ, err)
		}
		if v, _ := f.GetCellValue(resultExportSheet, ref); v == "" || strings.ContainsAny(v, `'"`) {
			t.Errorf("cell %s = %q, a number must not be text", ref, v)
		}
	}
	for r := 2; r <= len(xRows); r++ {
		cell, _ := excelize.CoordinatesToCellName(1, r)
		typ, err := f.GetCellType(resultExportSheet, cell)
		if err != nil || (typ != excelize.CellTypeSharedString && typ != excelize.CellTypeInlineString) {
			t.Errorf("cell %s type = %v (err %v), a text cell must be an explicit string", cell, typ, err)
		}
		if formula, _ := f.GetCellFormula(resultExportSheet, cell); formula != "" {
			t.Errorf("cell %s holds formula %q", cell, formula)
		}
	}
}

func TestResultExportFiltersByTryoutAndProgramAndFlagsIRT(t *testing.T) {
	h := newResultExportHarness(t)
	q1, q2 := h.question(t), h.question(t)
	weighted := h.tryoutOf(t, nil, q1)
	irt := h.tryoutOf(t, func(in *TryoutInput) { in.ScoringMode = model.ScoringIRT }, q2)
	ada := h.studentNamed(t, "Ada", weighted)
	h.enroll(t, model.User{ID: ada.UserID}, irt.ID)
	h.take(t, ada, weighted, map[uuid.UUID]string{q1.ID: optID(t, q1, "4")})
	h.take(t, ada, irt, map[uuid.UUID]string{q2.ID: optID(t, q2, "4")})

	only := csvRows(t, h.bytesOf(h.request(t, model.ExportTryoutResultsCSV, &irt.ID, nil)))
	if len(only) != 2 || only[1][1] != irt.Title || only[1][8] != "Ya" {
		t.Fatalf("irt tryout rows = %v, want one row flagged Ya", only)
	}
	program := weighted.ProgramID
	byProgram := csvRows(t, h.bytesOf(h.request(t, model.ExportTryoutResultsCSV, nil, &program)))
	if len(byProgram) != 3 {
		t.Fatalf("program rows = %d, want header + both tryouts of the program", len(byProgram))
	}
	none := uuid.New()
	if _, err := h.rs.PrepareExport(context.Background(), h.scopeA(), h.staff(model.RoleAdmin), nil, &none); !errors.Is(err, ErrProgramNotFound) {
		t.Errorf("unknown program err = %v, want program not found", err)
	}
	if _, err := h.rs.PrepareExport(context.Background(), h.scopeA(), h.staff(model.RoleAdmin), &none, nil); !errors.Is(err, ErrTryoutNotFound) {
		t.Errorf("unknown tryout err = %v, want tryout not found", err)
	}
	if _, err := h.rs.PrepareExport(context.Background(), h.scopeA(), ada, nil, nil); !errors.Is(err, ErrRuntimeForbidden) {
		t.Errorf("student err = %v, want forbidden", err)
	}
}

func TestResultExportRendererFailureIsRetriedThenFailed(t *testing.T) {
	h := newResultExportHarness(t)
	ctx := context.Background()
	admin := h.staff(model.RoleAdmin)
	e, err := h.exports.Request(ctx, h.scopeA(), admin, model.ExportTryoutResultsXLSX, json.RawMessage(`[1]`))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if err := h.exports.Run(ctx, h.tenant().ID, e.ID, 1, ExportMaxAttempts); err == nil {
		t.Fatal("first failed attempt must return the error so River retries")
	}
	if got, _ := h.exports.Get(ctx, h.scopeA(), admin, e.ID); got.Status != model.ExportQueued || got.ErrorCode != "render_failed" {
		t.Errorf("after attempt 1: %+v, want queued with render_failed", got)
	}
	if err := h.exports.Run(ctx, h.tenant().ID, e.ID, ExportMaxAttempts, ExportMaxAttempts); err != nil {
		t.Fatalf("last attempt must end the export without an error: %v", err)
	}
	if got, _ := h.exports.Get(ctx, h.scopeA(), admin, e.ID); got.Status != model.ExportFailed {
		t.Errorf("after the last attempt: %+v, want failed", got)
	}
}

func TestResultExportBeyondTheRowCapFailsVisibly(t *testing.T) {
	h := newResultExportHarness(t)
	q1 := h.question(t)
	tr := h.tryoutOf(t, nil, q1)
	for _, name := range []string{"Satu", "Dua"} {
		p := h.studentNamed(t, name, tr)
		h.take(t, p, tr, map[uuid.UUID]string{q1.ID: optID(t, q1, "4")})
	}
	h.csv.maxRows = 1
	ctx := context.Background()
	admin := h.staff(model.RoleAdmin)
	e, err := h.exports.Request(ctx, h.scopeA(), admin, model.ExportTryoutResultsCSV, ResultExportParams{TryoutID: &tr.ID})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if err := h.exports.Run(ctx, h.tenant().ID, e.ID, ExportMaxAttempts, ExportMaxAttempts); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, _ := h.exports.Get(ctx, h.scopeA(), admin, e.ID)
	if got.Status != model.ExportFailed || got.ErrorCode != ExportCodeTooManyRows {
		t.Errorf("export = %+v, want failed with too_many_rows", got)
	}
}

func TestResultStaffListFiltersByProgram(t *testing.T) {
	h := newResultHarness(t)
	q1 := h.question(t)
	tr := h.tryoutOf(t, nil, q1)
	_, p := h.enrolledStudentOf(t, tr)
	h.take(t, p, tr, map[uuid.UUID]string{q1.ID: optID(t, q1, "4")})
	ctx := context.Background()
	rows, total, err := h.rs.StaffResults(ctx, h.scopeA(), h.staff(model.RoleAdmin), StaffResultFilter{ProgramID: &tr.ProgramID})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Score != 100 || rows[0].Rank != 1 {
		t.Fatalf("program filter = %+v of %d (err %v), want the one result", rows, total, err)
	}
	other := uuid.New()
	rows, total, err = h.rs.StaffResults(ctx, h.scopeA(), h.staff(model.RoleAdmin), StaffResultFilter{ProgramID: &other})
	if err != nil || total != 0 || len(rows) != 0 {
		t.Errorf("another program = %d of %d (err %v), want none", len(rows), total, err)
	}
}

func TestNeutralizeFormula(t *testing.T) {
	for in, want := range map[string]string{
		"=1+1": "'=1+1", "+1": "'+1", "-1": "'-1", "@x": "'@x", "\tx": "'\tx", "\rx": "'\rx", "Ani": "Ani", "": "", "a=b": "a=b",
	} {
		if got := neutralizeFormula(in); got != want {
			t.Errorf("neutralizeFormula(%q) = %q, want %q", in, got, want)
		}
	}
}
