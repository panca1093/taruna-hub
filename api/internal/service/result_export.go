package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

const (
	ExportFormatCSV  = "csv"
	ExportFormatXLSX = "xlsx"

	// ResultExportMaxRows bounds the in-memory build; a larger report fails visibly instead of exhausting the worker.
	ResultExportMaxRows = 50000

	ExportCodeTooManyRows = "too_many_rows"

	resultExportSheet      = "Hasil"
	resultExportIRTChunk   = 1000
	resultExportCSVType    = "text/csv; charset=utf-8"
	resultExportXLSXType   = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	resultExportTimeLayout = "2006-01-02T15:04:05-07:00"
)

// jakarta is a fixed +07:00 zone so the worker image needs no tzdata; Indonesia's WIB has no daylight saving.
var jakarta = time.FixedZone("WIB", 7*60*60)

var resultExportHeader = []string{
	"Nama siswa", "Tryout", "Program", "Percobaan ke", "Skor total", "Rata-rata", "Peringkat", "Jumlah peserta",
	"Peringatan IRT", "Selesai pada",
}

// ResultExportParams is what the stored export row carries; the format is the kind.
type ResultExportParams struct {
	TryoutID  *uuid.UUID `json:"tryout_id,omitempty"`
	ProgramID *uuid.UUID `json:"program_id,omitempty"`
}

// PrepareExport checks the caller may export and that a named tryout or program is this tenant's, so a foreign id is
// a 404 and never a queued job that renders nothing.
func (s *ResultService) PrepareExport(ctx context.Context, scope HostScope, principal model.Principal, tryoutID, programID *uuid.UUID) (ResultExportParams, error) {
	tenantID, err := staffTenant(scope, principal)
	if err != nil {
		return ResultExportParams{}, err
	}
	if tryoutID != nil {
		if ok, err := s.repo.TryoutExists(ctx, s.db, tenantID, *tryoutID); err != nil {
			return ResultExportParams{}, err
		} else if !ok {
			return ResultExportParams{}, ErrTryoutNotFound
		}
	}
	if programID != nil {
		if ok, err := s.repo.ProgramExists(ctx, s.db, tenantID, *programID); err != nil {
			return ResultExportParams{}, err
		} else if !ok {
			return ResultExportParams{}, ErrProgramNotFound
		}
	}
	return ResultExportParams{TryoutID: tryoutID, ProgramID: programID}, nil
}

// ExportKindOf maps a requested format to its export kind.
func ExportKindOf(format string) (string, bool) {
	switch format {
	case ExportFormatCSV:
		return model.ExportTryoutResultsCSV, true
	case ExportFormatXLSX:
		return model.ExportTryoutResultsXLSX, true
	}
	return "", false
}

// ResultExportRenderer renders the tryout results report of one format. Both formats read the same rows, as of the
// export's cut-off.
type ResultExportRenderer struct {
	db      *pgxpool.Pool
	repo    *repository.ResultRepository
	xlsx    bool
	irtMin  int
	maxRows int
}

func NewResultExportRenderer(db *pgxpool.Pool, repo *repository.ResultRepository, format string, irtMin int) *ResultExportRenderer {
	if irtMin <= 0 {
		irtMin = IRTMinStudents
	}
	return &ResultExportRenderer{db: db, repo: repo, xlsx: format == ExportFormatXLSX, irtMin: irtMin, maxRows: ResultExportMaxRows}
}

func (r *ResultExportRenderer) Render(ctx context.Context, job model.Export) (ExportArtifact, error) {
	var params ResultExportParams
	if len(job.Params) > 0 {
		if err := json.Unmarshal(job.Params, &params); err != nil {
			return ExportArtifact{}, fmt.Errorf("decode result export params: %w", err)
		}
	}
	rows, err := r.repo.ExportRows(ctx, r.db, job.TenantID, repository.ExportFilter{
		TryoutID: params.TryoutID, ProgramID: params.ProgramID, CutOff: job.CutOffAt, Limit: r.maxRows + 1,
	})
	if err != nil {
		return ExportArtifact{}, err
	}
	if len(rows) > r.maxRows {
		return ExportArtifact{}, &ExportError{Code: ExportCodeTooManyRows, Err: fmt.Errorf("more than %d rows", r.maxRows)}
	}
	if err := r.flagIRT(ctx, job.TenantID, rows); err != nil {
		return ExportArtifact{}, err
	}
	table := make([][]any, 0, len(rows))
	for _, row := range rows {
		table = append(table, resultExportRow(row))
	}
	name := "hasil-tryout-" + job.CutOffAt.In(jakarta).Format("2006-01-02")
	if r.xlsx {
		data, err := buildResultXLSX(table)
		return ExportArtifact{Data: data, ContentType: resultExportXLSXType, FileName: name + ".xlsx"}, err
	}
	data, err := buildResultCSV(table)
	return ExportArtifact{Data: data, ContentType: resultExportCSVType, FileName: name + ".csv"}, err
}

// flagIRT fills IRTWarning for IRT-mode rows, in chunks so a large report never sends one huge ANY list.
func (r *ResultExportRenderer) flagIRT(ctx context.Context, tenantID uuid.UUID, rows []model.StaffResult) error {
	var ids []uuid.UUID
	for _, row := range rows {
		if row.ScoringMode == "irt" {
			ids = append(ids, row.SessionID)
		}
	}
	under := map[uuid.UUID]bool{}
	for start := 0; start < len(ids); start += resultExportIRTChunk {
		end := min(start+resultExportIRTChunk, len(ids))
		got, err := r.repo.IRTUnderThreshold(ctx, r.db, tenantID, ids[start:end], r.irtMin)
		if err != nil {
			return err
		}
		for id, flag := range got {
			under[id] = flag
		}
	}
	for i := range rows {
		rows[i].IRTWarning = under[rows[i].SessionID]
	}
	return nil
}

func resultExportRow(r model.StaffResult) []any {
	irt := "Tidak"
	if r.IRTWarning {
		irt = "Ya"
	}
	return []any{
		r.StudentName, r.TryoutTitle, r.ProgramCode, r.AttemptNumber, r.Score, r.Average, r.Rank, r.Participants, irt,
		r.CompletedAt.In(jakarta).Format(resultExportTimeLayout),
	}
}

// neutralizeFormula stops a spreadsheet from running a student-chosen text as a formula (CSV injection).
func neutralizeFormula(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func buildResultCSV(table [][]any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	if err := w.Write(resultExportHeader); err != nil {
		return nil, err
	}
	for _, row := range table {
		record := make([]string, len(row))
		for i, v := range row {
			switch v := v.(type) {
			case string:
				record[i] = neutralizeFormula(v)
			case int:
				record[i] = strconv.Itoa(v)
			case float64:
				record[i] = strconv.FormatFloat(v, 'f', -1, 64)
			}
		}
		if err := w.Write(record); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func buildResultXLSX(table [][]any) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", resultExportSheet); err != nil {
		return nil, err
	}
	for i, h := range resultExportHeader {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellStr(resultExportSheet, cell, h); err != nil {
			return nil, err
		}
	}
	for r, row := range table {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			var err error
			switch v := v.(type) {
			case string:
				err = f.SetCellStr(resultExportSheet, cell, neutralizeFormula(v))
			case int:
				err = f.SetCellInt(resultExportSheet, cell, int64(v))
			case float64:
				err = f.SetCellFloat(resultExportSheet, cell, v, -1, 64)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
