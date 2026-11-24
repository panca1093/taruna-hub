package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrQuestionNotFound   = errors.New("question not found")
	ErrQuestionStale      = errors.New("question changed by someone else")
	ErrQuestionReferenced = errors.New("question still referenced")
)

// Question field reasons; the transport turns each into the copy the editor shows.
const (
	ReasonQuestionType    = "question_type"
	ReasonQuestionContent = "question_content"
	ReasonQuestionImage   = "question_image"
	ReasonQuestionExpl    = "question_explanation"
	ReasonDifficulty      = "question_difficulty"
	ReasonProgramID       = "question_program"
	ReasonSubjectID       = "question_subject"
	ReasonChapterID       = "question_chapter"
	ReasonOptions         = "question_options"
	ReasonOptionLabel     = "question_option_label"
	ReasonOptionDuplicate = "question_option_duplicate"
	ReasonOptionForeign   = "question_option_foreign"
	ReasonOptionPoints    = "question_option_points"
	ReasonCorrect         = "question_correct"
	ReasonWeighted        = "question_weighted"
	ReasonCaseSensitive   = "question_case_sensitive"
	ReasonQueryLimit      = "question_limit"
	ReasonBaseVersion     = "question_base_version"
)

const (
	minOptions           = 2
	maxOptions           = 8
	maxContentLen        = 20000
	maxExplanationLen    = 20000
	maxOptionLabelLen    = 2000
	backfillBatch        = 500
	minIsianAnswers      = 1
	maxIsianAnswers      = 10
	maxIsianAnswerLen    = 200
	minMultiCorrect      = 2
	maxPoints            = 100
	defaultQuestionLimit = 50
	maxQuestionLimit     = 100
)

// markdownImage finds the URL of a markdown image in imported text, which may not carry a data: or javascript: URL.
var markdownImage = regexp.MustCompile(`!\[[^\]]*\]\(\s*([^)\s]*)`)

type QuestionOptionInput struct {
	ID     string
	Label  model.RichDoc
	Points *float64
}

type QuestionInput struct {
	ProgramID      uuid.UUID
	SubjectID      uuid.UUID
	ChapterID      *uuid.UUID
	Type           string
	Content        model.RichDoc
	Difficulty     string
	Explanation    model.RichDoc
	OptionWeighted bool
	CaseSensitive  bool
	Options        []QuestionOptionInput
	Correct        []string
	// BaseVersion is the version the editor started from; a mismatch is a 409, never a silent write.
	BaseVersion int
	// docRules is set by the service, never by the transport: what image src the documents may carry.
	docRules docRules
}

// QuestionFilter is the validated list query.
type QuestionFilter struct {
	ProgramID  *uuid.UUID
	SubjectID  *uuid.UUID
	ChapterID  *uuid.UUID
	Type       string
	Difficulty string
	Search     string
	Limit      int
	Offset     int
}

type QuestionService struct {
	db      *pgxpool.Pool
	repo    *repository.QuestionRepository
	assets  *AssetService
	runtime *repository.RuntimeRepository
}

// NewQuestionService takes the asset service that finalizes the images a question's documents point at.
func NewQuestionService(db *pgxpool.Pool, repo *repository.QuestionRepository, assets *AssetService) *QuestionService {
	return &QuestionService{db: db, repo: repo, assets: assets, runtime: repository.NewRuntimeRepository(db)}
}

// BackfillDocs gives every question version stored as text its documents and returns how many it converted. An
// instance that cannot take the lock, or a row that cannot be converted, is logged and passed over: reads fall back
// to the converted text, so neither stops the server from starting.
func (s *QuestionService) BackfillDocs(ctx context.Context, logger *slog.Logger) (int, error) {
	if logger == nil {
		logger = slog.Default()
	}
	result, err := s.repo.BackfillDocs(ctx, backfillBatch, convertLegacyVersion)
	if err != nil {
		return result.Converted, err
	}
	if !result.Locked {
		logger.Info("question document backfill skipped: another instance is running it")
	}
	for _, skipped := range result.Skipped {
		logger.Error("question version left as text", slog.String("version_id", skipped.ID.String()), slog.String("error", skipped.Err.Error()))
	}
	return result.Converted, nil
}

// convertLegacyVersion is the same conversion a read of that row applies.
func convertLegacyVersion(l repository.LegacyVersion) (repository.VersionDocs, error) {
	var options []model.QuestionOption
	if err := json.Unmarshal(l.Options, &options); err != nil {
		return repository.VersionDocs{}, fmt.Errorf("decode options: %w", err)
	}
	if options == nil {
		options = []model.QuestionOption{}
	}
	model.OptionsWithDocs(options)
	resolveOptions(options)
	return repository.VersionDocs{
		Content: docOfLegacyText(l.Content), Explanation: docOfLegacyText(l.Explanation), Options: options,
	}, nil
}

// questionTenant pins every query to the host's tenant and refuses a principal from another one.
func questionTenant(scope HostScope, principal model.Principal) (uuid.UUID, error) {
	if scope.Tenant.ID == uuid.Nil || principal.TenantID == nil || *principal.TenantID != scope.Tenant.ID {
		return uuid.Nil, ErrTenantNotFound
	}
	return scope.Tenant.ID, nil
}

func (s *QuestionService) List(ctx context.Context, scope HostScope, principal model.Principal, filter QuestionFilter) ([]model.Question, int, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	filter, err = validatedFilter(filter)
	if err != nil {
		return nil, 0, err
	}
	questions, total, err := s.repo.List(ctx, tenantID, repository.QuestionFilter{
		ProgramID: filter.ProgramID, SubjectID: filter.SubjectID, ChapterID: filter.ChapterID,
		Type: filter.Type, Difficulty: filter.Difficulty, Search: filter.Search,
		Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list questions: %w", err)
	}
	resolveQuestions(questions)
	return questions, total, nil
}

func (s *QuestionService) Get(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.Question, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Question{}, err
	}
	question, err := s.repo.Find(ctx, tenantID, id)
	if err != nil {
		return model.Question{}, questionError(err)
	}
	resolveQuestion(&question)
	if question.RescoreAttempts, err = s.runtime.CompletedSessionsWith(ctx, s.db, tenantID, id); err != nil {
		return model.Question{}, err
	}
	return question, nil
}

func (s *QuestionService) Create(ctx context.Context, scope HostScope, principal model.Principal, input QuestionInput) (model.Question, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Question{}, err
	}
	input, err = s.prepared(ctx, tenantID, input)
	if err != nil {
		return model.Question{}, err
	}
	input, err = validatedOptions(input, nil)
	if err != nil {
		return model.Question{}, err
	}
	var created model.Question
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		id, err := s.repo.CreateTx(ctx, tx, tenantID, principal.UserID, questionOf(input))
		if err != nil {
			return questionError(err)
		}
		if err := s.attachImages(ctx, tx, scope, principal, input); err != nil {
			return err
		}
		created, err = s.repo.FindCurrentTx(ctx, tx, tenantID, id, false)
		resolveQuestion(&created)
		return questionError(err)
	})
	if err != nil {
		return model.Question{}, err
	}
	return created, nil
}

func (s *QuestionService) Update(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, input QuestionInput) (model.Question, error) {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return model.Question{}, err
	}
	if input.BaseVersion < 1 {
		return model.Question{}, &InputError{Fields: map[string]string{"base_version": ReasonBaseVersion}}
	}
	// Look the question up before checking the body so another tenant's id answers 404 and leaks nothing.
	existing, err := s.repo.Find(ctx, tenantID, id)
	if err != nil {
		return model.Question{}, questionError(err)
	}
	resolveQuestion(&existing)
	input.docRules = docRules{srcs: questionSrcs(existing)}
	input, err = scalarOf(input)
	if err != nil {
		return model.Question{}, err
	}
	if err := s.checkRefs(ctx, tenantID, input); err != nil {
		return model.Question{}, err
	}

	var out model.Question
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.repo.FindCurrentTx(ctx, tx, tenantID, id, true)
		if err != nil {
			return questionError(err)
		}
		if input.BaseVersion != current.Version {
			return ErrQuestionStale
		}
		resolveQuestion(&current)
		input.docRules = docRules{srcs: questionSrcs(current)}
		input, err = validatedOptions(input, current.Options)
		if err != nil {
			return err
		}
		if sameContent(current, input) {
			out = current
			return nil
		}
		if err := s.attachImages(ctx, tx, scope, principal, input); err != nil {
			return err
		}
		next := current.Version + 1
		if err := s.repo.InsertVersionTx(ctx, tx, tenantID, id, next, principal.UserID, questionOf(input)); err != nil {
			return questionError(err)
		}
		if err := s.repo.TouchHeadTx(ctx, tx, tenantID, id, next); err != nil {
			return questionError(err)
		}
		out, err = s.repo.FindCurrentTx(ctx, tx, tenantID, id, false)
		if err != nil {
			return questionError(err)
		}
		resolveQuestion(&out)
		// Owner decision 2026-11-24: an edit reaches every tryout and attempt using the question, and taken ones are rescored.
		sessions, err := s.runtime.RepointQuestion(ctx, tx, tenantID, id, out.VersionID)
		if err != nil {
			return err
		}
		return rescoreSessions(ctx, tx, s.runtime, tenantID, sessions)
	})
	if err != nil {
		return model.Question{}, err
	}
	return out, nil
}

func (s *QuestionService) Delete(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) error {
	tenantID, err := questionTenant(scope, principal)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.repo.FindCurrentTx(ctx, tx, tenantID, id, true); err != nil {
			return questionError(err)
		}
		return questionError(s.repo.DeleteTx(ctx, tx, tenantID, id))
	})
}

func (s *QuestionService) prepared(ctx context.Context, tenantID uuid.UUID, input QuestionInput) (QuestionInput, error) {
	input, err := scalarOf(input)
	if err != nil {
		return QuestionInput{}, err
	}
	if err := s.checkRefs(ctx, tenantID, input); err != nil {
		return QuestionInput{}, err
	}
	return input, nil
}

// Composite foreign keys only catch the tenant half of program/subject/chapter consistency.
func (s *QuestionService) checkRefs(ctx context.Context, tenantID uuid.UUID, input QuestionInput) error {
	fields := map[string]string{}
	ok, err := s.repo.ProgramExists(ctx, tenantID, input.ProgramID)
	if err != nil {
		return err
	}
	if !ok {
		fields["program_id"] = ReasonProgramID
	} else {
		ok, err = s.repo.SubjectInProgram(ctx, tenantID, input.ProgramID, input.SubjectID)
		if err != nil {
			return err
		}
		if !ok {
			fields["subject_id"] = ReasonSubjectID
		}
	}
	if input.ChapterID != nil && len(fields) == 0 {
		ok, err := s.repo.ChapterInSubject(ctx, tenantID, input.SubjectID, *input.ChapterID)
		if err != nil {
			return err
		}
		if !ok {
			fields["chapter_id"] = ReasonChapterID
		}
	}
	if len(fields) > 0 {
		return &InputError{Fields: fields}
	}
	return nil
}

// scalarOf normalises the documents and checks everything that does not depend on the options.
func scalarOf(input QuestionInput) (QuestionInput, error) {
	fields := map[string]string{}

	if !knownType(input.Type) {
		fields["type"] = ReasonQuestionType
	}
	var reason string
	if input.Content, reason = validatedDocWith(input.Content, true, maxContentLen, ReasonQuestionContent, input.docRules); reason != "" {
		fields["content"] = reason
	}
	if input.Explanation, reason = validatedDocWith(input.Explanation, false, maxExplanationLen, ReasonQuestionExpl, input.docRules); reason != "" {
		fields["explanation"] = reason
	}
	if !knownDifficulty(input.Difficulty) {
		fields["difficulty"] = ReasonDifficulty
	}
	if input.ProgramID == uuid.Nil {
		fields["program_id"] = ReasonProgramID
	}
	if input.SubjectID == uuid.Nil {
		fields["subject_id"] = ReasonSubjectID
	}
	if input.CaseSensitive && input.Type != model.QuestionIsian {
		fields["case_sensitive"] = ReasonCaseSensitive
	}
	if len(fields) > 0 {
		return QuestionInput{}, &InputError{Fields: fields}
	}
	return input, nil
}

func validatedOptions(input QuestionInput, existing []model.QuestionOption) (QuestionInput, error) {
	fields := map[string]string{}
	weighted := input.OptionWeighted

	if input.OptionWeighted && input.Type != model.QuestionPG {
		fields["option_weighted"] = ReasonWeighted
		weighted = false
	}

	switch input.Type {
	case model.QuestionIsian:
		if len(input.Options) > 0 {
			fields["options"] = ReasonOptions
		}
		input.Options = nil
		answers, ok := isianAnswers(input)
		if !ok {
			fields["correct"] = ReasonCorrect
		}
		input.Correct = answers
	case model.QuestionTrueFalse:
		fixed := []QuestionOptionInput{
			{ID: "benar", Label: model.DocFromLegacyText("Benar")},
			{ID: "salah", Label: model.DocFromLegacyText("Salah")},
		}
		if len(input.Options) > 0 && !sameTrueFalse(input.Options) {
			fields["options"] = ReasonOptions
		}
		input.Options = fixed
		switch {
		case len(input.Correct) != 1:
			fields["correct"] = ReasonCorrect
		case input.Correct[0] != "benar" && input.Correct[0] != "salah":
			fields["correct"] = ReasonCorrect
		}
	default:
		options, remap, _ := resolvedOptions(input.Options, existing, fields, input.docRules)
		if len(options) < minOptions || len(options) > maxOptions {
			fields["options"] = ReasonOptions
		}
		input.Options = options
		input.Correct = remapCorrect(input.Correct, remap)

		idSet := make(map[string]bool, len(options))
		for _, option := range options {
			idSet[option.ID] = true
		}
		switch input.Type {
		case model.QuestionPG:
			if weighted {
				if len(input.Correct) > 0 {
					fields["correct"] = ReasonCorrect
				}
			} else if len(input.Correct) != 1 || !idSet[input.Correct[0]] {
				fields["correct"] = ReasonCorrect
			}
		case model.QuestionPGMulti:
			if !validMultiCorrect(input.Correct, idSet) {
				fields["correct"] = ReasonCorrect
			}
		}
		if weighted {
			checkPoints(input.Options, fields)
		} else {
			for i, option := range input.Options {
				if option.Points != nil {
					fields[fmt.Sprintf("options[%d].points", i)] = ReasonOptionPoints
				}
			}
		}
	}

	if len(fields) > 0 {
		return QuestionInput{}, &InputError{Fields: fields}
	}
	return input, nil
}

// Placeholder ids are noted so `correct` keeps pointing at the option the editor meant.
func resolvedOptions(input []QuestionOptionInput, existing []model.QuestionOption, fields map[string]string, rules docRules) ([]QuestionOptionInput, map[string]string, bool) {
	owned := make(map[string]bool, len(existing))
	for _, option := range existing {
		owned[option.ID] = true
	}
	out := make([]QuestionOptionInput, 0, len(input))
	remap := map[string]string{}
	seenLabels := make(map[string]bool, len(input))
	seenIDs := make(map[string]bool, len(input))
	ok := true
	for i, option := range input {
		path := fmt.Sprintf("options[%d]", i)
		doc, reason := validatedDocWith(option.Label, true, maxOptionLabelLen, ReasonOptionLabel, rules)
		option.Label = doc
		label := strings.ToLower(strings.TrimSpace(model.DocText(doc)))
		if label == "" {
			if raw, err := json.Marshal(doc); err == nil {
				label = strings.ToLower(string(raw))
			}
		}
		switch {
		case reason != "":
			fields[path+".label"] = reason
			ok = false
		case seenLabels[label]:
			fields[path+".label"] = ReasonOptionDuplicate
			ok = false
		}
		seenLabels[label] = true

		id := strings.TrimSpace(option.ID)
		switch {
		case id == "":
			option.ID = uuid.NewString()
		case owned[id]:
			option.ID = id
		case isUUID(id):
			fields[path+".id"] = ReasonOptionForeign
			ok = false
		default:
			// A client placeholder such as "a": the option is new, and `correct` may name it.
			remap[id] = uuid.NewString()
			option.ID = remap[id]
		}
		if seenIDs[option.ID] {
			fields[path+".id"] = ReasonOptionDuplicate
			ok = false
		}
		seenIDs[option.ID] = true
		out = append(out, option)
	}
	return out, remap, ok
}

func remapCorrect(correct []string, remap map[string]string) []string {
	if len(remap) == 0 {
		return correct
	}
	out := make([]string, 0, len(correct))
	for _, entry := range correct {
		if id, ok := remap[strings.TrimSpace(entry)]; ok {
			out = append(out, id)
			continue
		}
		out = append(out, strings.TrimSpace(entry))
	}
	return out
}

func validMultiCorrect(correct []string, options map[string]bool) bool {
	if len(correct) < minMultiCorrect {
		return false
	}
	seen := make(map[string]bool, len(correct))
	for _, id := range correct {
		if !options[id] || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

// isianAnswers trims the accepted answers and refuses blanks and duplicates under the case rule.
func isianAnswers(input QuestionInput) ([]string, bool) {
	if len(input.Correct) < minIsianAnswers || len(input.Correct) > maxIsianAnswers {
		return nil, false
	}
	out := make([]string, 0, len(input.Correct))
	seen := make(map[string]bool, len(input.Correct))
	for _, answer := range input.Correct {
		answer = strings.TrimSpace(answer)
		if n := utf8.RuneCountInString(answer); n < 1 || n > maxIsianAnswerLen {
			return nil, false
		}
		key := answer
		if !input.CaseSensitive {
			key = strings.ToLower(answer)
		}
		if seen[key] {
			return nil, false
		}
		seen[key] = true
		out = append(out, answer)
	}
	return out, true
}

func checkPoints(options []QuestionOptionInput, fields map[string]string) {
	for i, option := range options {
		if option.Points == nil {
			continue
		}
		points := *option.Points
		if points < 0 || points > maxPoints || math.Abs(points*100-math.Round(points*100)) > 1e-9 {
			fields[fmt.Sprintf("options[%d].points", i)] = ReasonOptionPoints
		}
	}
}

func sameTrueFalse(options []QuestionOptionInput) bool {
	if len(options) != 2 {
		return false
	}
	want := map[string]string{"benar": "benar", "salah": "salah"}
	for _, option := range options {
		label, ok := want[strings.TrimSpace(option.ID)]
		if !ok || option.Points != nil || !strings.EqualFold(model.DocText(option.Label), label) {
			return false
		}
		delete(want, option.ID)
	}
	return len(want) == 0
}

// questionSrcs is every image src the version already holds, which an update may keep.
func questionSrcs(q model.Question) map[string]bool {
	docs := []model.RichDoc{q.Content, q.Explanation}
	for _, option := range q.Options {
		docs = append(docs, option.Doc)
	}
	return docSrcs(docs...)
}

// sameContent is the "saving an unchanged question writes nothing" rule.
func sameContent(current model.Question, input QuestionInput) bool {
	if current.ProgramID != input.ProgramID || current.SubjectID != input.SubjectID ||
		!sameUUID(current.ChapterID, input.ChapterID) {
		return false
	}
	if current.Type != input.Type || !sameDoc(current.Content, input.Content) || current.Difficulty != input.Difficulty ||
		!sameDoc(current.Explanation, input.Explanation) || current.OptionWeighted != input.OptionWeighted ||
		current.CaseSensitive != input.CaseSensitive {
		return false
	}
	if len(current.Options) != len(input.Options) {
		return false
	}
	for i := range current.Options {
		before, after := current.Options[i], input.Options[i]
		if before.ID != after.ID || !sameDoc(before.Doc, after.Label) {
			return false
		}
		if (before.Points == nil) != (after.Points == nil) || (before.Points != nil && *before.Points != *after.Points) {
			return false
		}
	}
	return sameSet(current.Correct, input.Correct)
}

// sameDoc compares canonical JSON, which both normalised and stored documents marshal to identically.
func sameDoc(a, b model.RichDoc) bool {
	left, errA := json.Marshal(a)
	right, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(left) == string(right)
}

func sameUUID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, entry := range a {
		seen[entry]++
	}
	for _, entry := range b {
		seen[entry]--
		if seen[entry] < 0 {
			return false
		}
	}
	return true
}

func validatedFilter(filter QuestionFilter) (QuestionFilter, error) {
	fields := map[string]string{}
	if filter.Limit == 0 {
		filter.Limit = defaultQuestionLimit
	}
	if filter.Limit < 1 || filter.Limit > maxQuestionLimit {
		fields["limit"] = ReasonQueryLimit
	}
	if filter.Offset < 0 {
		fields["offset"] = ReasonQueryLimit
	}
	if filter.Type != "" && !knownType(filter.Type) {
		fields["type"] = ReasonQuestionType
	}
	if filter.Difficulty != "" && !knownDifficulty(filter.Difficulty) {
		fields["difficulty"] = ReasonDifficulty
	}
	if len(fields) > 0 {
		return QuestionFilter{}, &InputError{Fields: fields}
	}
	return filter, nil
}

func questionOf(input QuestionInput) model.Question {
	out := model.Question{
		ProgramID: input.ProgramID, SubjectID: input.SubjectID, ChapterID: input.ChapterID,
		Type: input.Type, Content: input.Content, Difficulty: input.Difficulty,
		Explanation: input.Explanation, OptionWeighted: input.OptionWeighted,
		CaseSensitive: input.CaseSensitive, Correct: append([]string{}, input.Correct...),
		Options: make([]model.QuestionOption, 0, len(input.Options)),
	}
	for _, option := range input.Options {
		out.Options = append(out.Options, model.QuestionOption{
			ID: option.ID, Label: model.DocLegacyText(option.Label), Doc: option.Label, Points: option.Points,
		})
	}
	return out
}

func questionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrQuestionNotFound
	case errors.Is(err, repository.ErrReferenced):
		return ErrQuestionReferenced
	case errors.Is(err, repository.ErrScopeRequired):
		return ErrTenantNotFound
	}
	return err
}

func knownType(value string) bool {
	switch value {
	case model.QuestionPG, model.QuestionPGMulti, model.QuestionIsian, model.QuestionTrueFalse:
		return true
	}
	return false
}

func knownDifficulty(value string) bool {
	switch value {
	case model.DifficultyEasy, model.DifficultyMedium, model.DifficultyHard:
		return true
	}
	return false
}

func isUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func unsupportedImage(text string) bool {
	for _, match := range markdownImage.FindAllStringSubmatch(text, -1) {
		url := strings.ToLower(match[1])
		if strings.HasPrefix(url, "data:") || strings.HasPrefix(url, "javascript:") {
			return true
		}
	}
	return false
}

// attachImages finalizes, in the version's own transaction, every question_image the documents point at; an asset
// that is unknown, another tenant's or of another purpose names the field that holds it.
func (s *QuestionService) attachImages(ctx context.Context, tx pgx.Tx, scope HostScope, principal model.Principal, input QuestionInput) error {
	fields := map[string]string{}
	attach := func(path string, doc model.RichDoc) error {
		for _, id := range model.DocAssetIDs(doc) {
			if s.assets == nil {
				fields[path] = ReasonDocAsset
				return nil
			}
			if _, err := s.assets.FinalizePurposeTx(ctx, tx, scope, principal, id, model.PurposeQuestionImage); err != nil {
				if errors.Is(err, ErrAssetNotFound) {
					fields[path] = ReasonDocAsset
					return nil
				}
				return err
			}
		}
		return nil
	}
	if err := attach("content", input.Content); err != nil {
		return err
	}
	if err := attach("explanation", input.Explanation); err != nil {
		return err
	}
	for i, option := range input.Options {
		if err := attach(fmt.Sprintf("options[%d].label", i), option.Label); err != nil {
			return err
		}
	}
	if len(fields) > 0 {
		return &InputError{Fields: fields}
	}
	return nil
}

func (s *QuestionService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
