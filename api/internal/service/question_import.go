package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

// Import reasons; the transport turns each into the copy the upload dialog shows.
const (
	ReasonImportFile   = "import_file"
	ReasonImportEmpty  = "import_empty"
	ReasonImportRows   = "import_rows"
	ReasonImportHeader = "import_header"
	ReasonImportSize   = "import_size"
	ReasonImportFormat = "import_format"
	ReasonImportBool   = "import_bool"
)

// ImportFileError rejects the whole file: the document itself is unusable, not one row of it.
type ImportFileError struct{ Reason string }

func (e *ImportFileError) Error() string { return "import file: " + e.Reason }

// ImportRowsError names the selected rows that cannot be committed.
type ImportRowsError struct{ Rows []int }

func (e *ImportRowsError) Error() string {
	rows := make([]string, 0, len(e.Rows))
	for _, row := range e.Rows {
		rows = append(rows, strconv.Itoa(row))
	}
	return "import rows: " + strings.Join(rows, ", ")
}

// ImportTemplate is a generated template file.
type ImportTemplate struct {
	FileName    string
	ContentType string
	Data        []byte
}

type importRow struct {
	model.ImportRow
	input QuestionInput
}

// importRefs is the tenant's curriculum keyed by name, so a row resolves its names without SQL.
type importRefs struct {
	ordered  []repository.CurriculumRef
	programs map[string]uuid.UUID
	subjects map[uuid.UUID]map[string]uuid.UUID
	chapters map[uuid.UUID]map[string]uuid.UUID
}

func importRefsOf(refs []repository.CurriculumRef) importRefs {
	out := importRefs{
		ordered:  refs,
		programs: map[string]uuid.UUID{},
		subjects: map[uuid.UUID]map[string]uuid.UUID{},
		chapters: map[uuid.UUID]map[string]uuid.UUID{},
	}
	remember := func(byParent map[uuid.UUID]map[string]uuid.UUID, parent, id uuid.UUID, name string) {
		names := byParent[parent]
		if names == nil {
			names = map[string]uuid.UUID{}
			byParent[parent] = names
		}
		if _, ok := names[name]; !ok {
			names[name] = id
		}
	}
	for _, ref := range refs {
		name := strings.ToLower(strings.TrimSpace(ref.Name))
		switch ref.Kind {
		case "program":
			if _, ok := out.programs[name]; !ok {
				out.programs[name] = ref.ID
			}
		case "subject":
			if ref.ParentID != nil {
				remember(out.subjects, *ref.ParentID, ref.ID, name)
			}
		case "chapter":
			if ref.ParentID != nil {
				remember(out.chapters, *ref.ParentID, ref.ID, name)
			}
		}
	}
	return out
}

func (r importRefs) program(name string) (uuid.UUID, bool) {
	id, ok := r.programs[strings.ToLower(strings.TrimSpace(name))]
	return id, ok
}

func (r importRefs) subject(programID uuid.UUID, name string) (uuid.UUID, bool) {
	id, ok := r.subjects[programID][strings.ToLower(strings.TrimSpace(name))]
	return id, ok
}

func (r importRefs) chapter(subjectID uuid.UUID, name string) (uuid.UUID, bool) {
	id, ok := r.chapters[subjectID][strings.ToLower(strings.TrimSpace(name))]
	return id, ok
}

// templateNames is the first program, its first subject and that subject's first chapter.
func (r importRefs) templateNames() (string, string, string) {
	for _, ref := range r.ordered {
		if ref.Kind != "program" {
			continue
		}
		for _, subject := range r.ordered {
			if subject.Kind != "subject" || subject.ParentID == nil || *subject.ParentID != ref.ID {
				continue
			}
			for _, chapter := range r.ordered {
				if chapter.Kind == "chapter" && chapter.ParentID != nil && *chapter.ParentID == subject.ID {
					return ref.Name, subject.Name, chapter.Name
				}
			}
			return ref.Name, subject.Name, ""
		}
		return ref.Name, "", ""
	}
	return "", "", ""
}

func (s *QuestionService) importRefs(ctx context.Context, tenantID uuid.UUID) (importRefs, error) {
	refs, err := s.repo.CurriculumRefs(ctx, tenantID)
	if err != nil {
		return importRefs{}, err
	}
	return importRefsOf(refs), nil
}

func (s *QuestionService) PreviewImport(ctx context.Context, scope HostScope, principal model.Principal, fileName string, data []byte) (model.ImportPreview, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.ImportPreview{}, err
	}
	sheet, err := parseImportFile(fileName, data)
	if err != nil {
		return model.ImportPreview{}, err
	}
	rows, err := s.importRows(ctx, tenantID, sheet)
	if err != nil {
		return model.ImportPreview{}, err
	}
	preview := model.ImportPreview{
		FileName: filepath.Base(fileName),
		Format:   sheet.format,
		Total:    len(rows),
		Rows:     make([]model.ImportRow, 0, len(rows)),
	}
	for _, row := range rows {
		preview.Rows = append(preview.Rows, row.ImportRow)
		if row.Valid {
			preview.Valid++
		} else {
			preview.Invalid++
		}
	}
	return preview, nil
}

func (s *QuestionService) CommitImport(ctx context.Context, scope HostScope, principal model.Principal, fileName string, data []byte, selected []int) (model.ImportCommit, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.ImportCommit{}, err
	}
	sheet, err := parseImportFile(fileName, data)
	if err != nil {
		return model.ImportCommit{}, err
	}
	rows, err := s.importRows(ctx, tenantID, sheet)
	if err != nil {
		return model.ImportCommit{}, err
	}
	chosen, err := selectedImportRows(rows, selected)
	if err != nil {
		return model.ImportCommit{}, err
	}

	digest := importDigest(data)
	commit := model.ImportCommit{QuestionIDs: []uuid.UUID{}}
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		for _, at := range chosen {
			key := digest + ":" + strconv.Itoa(rows[at].Row)
			id, created, err := s.repo.ImportCreateTx(ctx, tx, tenantID, principal.UserID, key, questionOf(rows[at].input))
			if err != nil {
				return questionError(err)
			}
			if created {
				commit.Created++
				commit.QuestionIDs = append(commit.QuestionIDs, id)
				continue
			}
			commit.AlreadyImported++
		}
		return nil
	})
	if err != nil {
		return model.ImportCommit{}, err
	}
	return commit, nil
}

// selectedImportRows refuses an empty, unknown, repeated or invalid selection before anything is written.
func selectedImportRows(rows []importRow, selected []int) ([]int, error) {
	if len(selected) == 0 {
		return nil, &InputError{Fields: map[string]string{"rows": ReasonImportRows}}
	}
	seen := make(map[int]bool, len(selected))
	chosen := make([]int, 0, len(selected))
	invalid := []int{}
	for _, number := range selected {
		if number < 1 || number > len(rows) || seen[number] {
			return nil, &InputError{Fields: map[string]string{"rows": ReasonImportRows}}
		}
		seen[number] = true
		if !rows[number-1].Valid {
			invalid = append(invalid, number)
			continue
		}
		chosen = append(chosen, number-1)
	}
	if len(invalid) > 0 {
		sort.Ints(invalid)
		return nil, &ImportRowsError{Rows: invalid}
	}
	sort.Ints(chosen)
	return chosen, nil
}

func (s *QuestionService) ImportTemplate(ctx context.Context, scope HostScope, principal model.Principal, format string) (ImportTemplate, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return ImportTemplate{}, err
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "xlsx"
	}
	if format != "xlsx" && format != "csv" && format != "docx" {
		return ImportTemplate{}, &InputError{Fields: map[string]string{"format": ReasonImportFormat}}
	}
	refs, err := s.importRefs(ctx, tenantID)
	if err != nil {
		return ImportTemplate{}, err
	}

	var data []byte
	rows := templateRows(refs)
	switch format {
	case "csv":
		data, err = writeCSV(rows)
	case "docx":
		data, err = writeDOCX(rows)
	default:
		data, err = writeXLSX(rows)
	}
	if err != nil {
		return ImportTemplate{}, fmt.Errorf("build %s template: %w", format, err)
	}
	return ImportTemplate{
		FileName:    "template-soal." + format,
		ContentType: importContentType(format),
		Data:        data,
	}, nil
}

func importContentType(format string) string {
	switch format {
	case "csv":
		return "text/csv; charset=utf-8"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}

// templateRows is the header plus one pg row and one isian row, named after the tenant's curriculum.
func templateRows(refs importRefs) [][]string {
	program, subject, chapter := refs.templateNames()
	row := func(values map[string]string) []string {
		out := make([]string, len(importColumns))
		for at, column := range importColumns {
			out[at] = values[column]
		}
		return out
	}
	return [][]string{
		append([]string{}, importColumns...),
		row(map[string]string{
			"type": "pg", "program": program, "subject": subject, "chapter": chapter,
			"difficulty": "easy", "content": "Hasil dari 2 + 2 adalah ...", "explanation": "2 + 2 = 4.",
			"option_a": "3", "option_b": "4", "correct": "B",
		}),
		row(map[string]string{
			"type": "isian", "program": program, "subject": subject, "chapter": chapter,
			"difficulty": "medium", "content": "Ibu kota Jawa Barat adalah ...", "explanation": "Bandung.",
			"correct": "Bandung", "case_sensitive": "no",
		}),
	}
}

func (s *QuestionService) importRows(ctx context.Context, tenantID uuid.UUID, sheet importSheet) ([]importRow, error) {
	refs, err := s.importRefs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]importRow, 0, len(sheet.rows))
	for at, cells := range sheet.rows {
		out = append(out, importRowOf(sheet, refs, at+1, cells))
	}
	return out, nil
}

// importRowOf builds the create endpoint's QuestionInput, judges it with the shared rules, names the columns.
func importRowOf(sheet importSheet, refs importRefs, number int, cells []string) importRow {
	fields := map[string]string{}
	note := func(column, reason string) {
		if _, ok := fields[column]; !ok {
			fields[column] = reason
		}
	}
	cell := func(name string) string { return strings.TrimSpace(sheet.cell(cells, name)) }

	input := QuestionInput{
		Type:        strings.ToLower(cell("type")),
		Content:     sheet.cell(cells, "content"),
		Explanation: sheet.cell(cells, "explanation"),
		Difficulty:  importDifficulty(cell("difficulty")),
	}

	if programID, ok := refs.program(cell("program")); !ok {
		note("program", ReasonProgramID)
	} else {
		input.ProgramID = programID
		subjectID, ok := refs.subject(programID, cell("subject"))
		if !ok {
			note("subject", ReasonSubjectID)
		} else {
			input.SubjectID = subjectID
			if name := cell("chapter"); name != "" {
				if chapterID, ok := refs.chapter(subjectID, name); !ok {
					note("chapter", ReasonChapterID)
				} else {
					input.ChapterID = &chapterID
				}
			}
		}
	}

	optionColumns := []string{}
	for _, column := range importColumns {
		if !strings.HasPrefix(column, "option_") {
			continue
		}
		label := cell(column)
		if label == "" {
			continue
		}
		input.Options = append(input.Options, QuestionOptionInput{
			ID: strings.TrimPrefix(column, "option_"), Label: label,
		})
		optionColumns = append(optionColumns, column)
	}

	input.Correct = importCorrect(input.Type, cell("correct"))
	if raw := cell("case_sensitive"); raw != "" {
		sensitive, ok := importYesNo(raw)
		if !ok {
			note("case_sensitive", ReasonImportBool)
		} else {
			input.CaseSensitive = sensitive
		}
	}

	row := importRow{
		ImportRow: model.ImportRow{
			Row: number,
			Preview: &model.ImportRowPreview{
				Type:       input.Type,
				Content:    strings.TrimSpace(input.Content),
				Program:    cell("program"),
				Subject:    cell("subject"),
				Difficulty: input.Difficulty,
			},
		},
		input: input,
	}

	if scalars, err := scalarOf(input); err != nil {
		importFieldErrors(err, note, optionColumns)
	} else if validated, err := validatedOptions(scalars, nil); err != nil {
		importFieldErrors(err, note, optionColumns)
	} else {
		row.input = validated
	}

	row.Valid = len(fields) == 0
	row.Errors = make([]model.ImportRowError, 0, len(fields))
	for column, reason := range fields {
		row.Errors = append(row.Errors, model.ImportRowError{Field: column, Reason: reason})
	}
	sort.Slice(row.Errors, func(i, j int) bool { return row.Errors[i].Field < row.Errors[j].Field })
	return row
}

func importFieldErrors(err error, note func(column, reason string), optionColumns []string) {
	var input *InputError
	if !errors.As(err, &input) {
		note("file", ReasonImportFile)
		return
	}
	for path, reason := range input.Fields {
		column := importColumn(path, optionColumns)
		// The row's own name resolution already named these, and did it more precisely.
		if column == "program" || column == "subject" || column == "chapter" {
			continue
		}
		note(column, reason)
	}
}

// importColumn names the spreadsheet column a service field path came from.
func importColumn(path string, optionColumns []string) string {
	firstOption := "option_a"
	if len(optionColumns) > 0 {
		firstOption = optionColumns[0]
	}
	if rest, ok := strings.CutPrefix(path, "options["); ok {
		if at, err := strconv.Atoi(strings.SplitN(rest, "]", 2)[0]); err == nil && at >= 0 && at < len(optionColumns) {
			return optionColumns[at]
		}
		return firstOption
	}
	switch path {
	case "program_id":
		return "program"
	case "subject_id":
		return "subject"
	case "chapter_id":
		return "chapter"
	case "options":
		return firstOption
	}
	return path
}

func importDifficulty(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "mudah":
		return model.DifficultyEasy
	case "sedang":
		return model.DifficultyMedium
	case "sulit":
		return model.DifficultyHard
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func importCorrect(kind, raw string) []string {
	raw = strings.TrimSpace(raw)
	switch kind {
	case model.QuestionIsian:
		if raw == "" {
			return nil
		}
		parts := strings.Split(raw, "|")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			out = append(out, strings.TrimSpace(part))
		}
		return out
	case model.QuestionTrueFalse:
		switch strings.ToLower(raw) {
		case "benar", "true":
			return []string{"benar"}
		case "salah", "false":
			return []string{"salah"}
		case "":
			return nil
		}
		return []string{strings.ToLower(raw)}
	default:
		out := []string{}
		for _, letter := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' }) {
			if letter = strings.ToLower(strings.TrimSpace(letter)); letter != "" {
				out = append(out, letter)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
}

func importYesNo(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "yes", "true", "1":
		return true, true
	case "no", "false", "0":
		return false, true
	}
	return false, false
}

func importDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:32]
}
