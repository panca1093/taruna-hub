package service

import (
	"context"
	"errors"
	"fmt"
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
	ErrProgramNotFound   = errors.New("program not found")
	ErrProgramCodeTaken  = errors.New("program code taken")
	ErrDuplicateName     = errors.New("name already used")
	ErrProgramReferenced = errors.New("program still referenced")
	// The narrow forms let the 409 name the level of the tree that is still in use.
	ErrChapterReferenced = fmt.Errorf("%w: chapter", ErrProgramReferenced)
	ErrSubjectReferenced = fmt.Errorf("%w: subject", ErrProgramReferenced)
)

// Curriculum field reasons; the transport turns each into the copy the form shows.
const (
	ReasonCode        = "code_rule"
	ReasonNameLength  = "name_length"
	ReasonTooLong     = "too_long"
	ReasonDuplicate   = "duplicate_name"
	ReasonLimit       = "limit"
	ReasonForeignItem = "foreign_item"
)

const (
	minCodeLen = 2
	maxCodeLen = 12
	maxNameLen = 100
	maxDescLen = 500
	// Bounds keep one program's form usable; a program above them is a data-entry mistake.
	maxSubjectsPerProgram = 50
	maxChaptersPerSubject = 200
)

var codePattern = regexp.MustCompile(`^[A-Z0-9_-]+$`)

// ProgramInput is one program as the caller wants it to look; a nil ID means "create this item".
type ProgramInput struct {
	Code        string
	Name        string
	Description string
	Subjects    []SubjectInput
}

type SubjectInput struct {
	ID       uuid.UUID
	Name     string
	Chapters []ChapterInput
}

type ChapterInput struct {
	ID   uuid.UUID
	Name string
}

type CurriculumService struct {
	db   *pgxpool.Pool
	repo *repository.CurriculumRepository
}

func NewCurriculumService(db *pgxpool.Pool, repo *repository.CurriculumRepository) *CurriculumService {
	return &CurriculumService{db: db, repo: repo}
}

// curriculumTenant pins every query to the host's tenant and refuses a principal from another one.
func curriculumTenant(scope HostScope, principal model.Principal) (uuid.UUID, error) {
	if scope.Platform || scope.Tenant.ID == uuid.Nil || principal.TenantID == nil || *principal.TenantID != scope.Tenant.ID {
		return uuid.Nil, ErrTenantNotFound
	}
	return scope.Tenant.ID, nil
}

func (s *CurriculumService) List(ctx context.Context, scope HostScope, principal model.Principal) ([]model.ProgramTree, error) {
	tenantID, err := curriculumTenant(scope, principal)
	if err != nil {
		return nil, err
	}
	trees, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list programs: %w", err)
	}
	return trees, nil
}

func (s *CurriculumService) Create(ctx context.Context, scope HostScope, principal model.Principal, input ProgramInput) (model.ProgramTree, error) {
	tenantID, err := curriculumTenant(scope, principal)
	if err != nil {
		return model.ProgramTree{}, err
	}
	input, err = validatedProgram(input)
	if err != nil {
		return model.ProgramTree{}, err
	}
	tree, err := s.repo.CreateTree(ctx, tenantID, treeOf(input))
	if err != nil {
		return model.ProgramTree{}, programError(err)
	}
	return tree, nil
}

// Replace rewrites the tree the body names, inside one transaction with the program row locked, so
// two admins saving at once cannot interleave their diffs.
func (s *CurriculumService) Replace(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, input ProgramInput) (model.ProgramTree, error) {
	tenantID, err := curriculumTenant(scope, principal)
	if err != nil {
		return model.ProgramTree{}, err
	}
	input, err = validatedProgram(input)
	if err != nil {
		return model.ProgramTree{}, err
	}
	var out model.ProgramTree
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.repo.FindTreeTx(ctx, tx, tenantID, id, true)
		if err != nil {
			return programError(err)
		}
		if inputErr := foreignItems(current, input); inputErr != nil {
			return inputErr
		}
		out, err = s.repo.ReplaceTreeTx(ctx, tx, tenantID, id, treeOf(input))
		return programError(err)
	})
	return out, err
}

// Delete removes the tree bottom-up in one transaction; anything still referencing it answers 409
// and rolls the whole delete back.
func (s *CurriculumService) Delete(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) error {
	tenantID, err := curriculumTenant(scope, principal)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := s.repo.FindTreeTx(ctx, tx, tenantID, id, true); err != nil {
			return programError(err)
		}
		if err := s.repo.DeleteChaptersTx(ctx, tx, tenantID, id); err != nil {
			return programError(err)
		}
		if err := s.repo.DeleteSubjectsTx(ctx, tx, tenantID, id); err != nil {
			return programError(err)
		}
		return programError(s.repo.DeleteProgramTx(ctx, tx, tenantID, id))
	})
}

// validatedProgram normalizes and checks every rule, naming each failing field by its wire path.
func validatedProgram(input ProgramInput) (ProgramInput, error) {
	fields := map[string]string{}
	input.Code = strings.ToUpper(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)

	if len(input.Code) < minCodeLen || len(input.Code) > maxCodeLen || !codePattern.MatchString(input.Code) {
		fields["code"] = ReasonCode
	}
	if n := utf8.RuneCountInString(input.Name); n < 1 || n > maxNameLen {
		fields["name"] = ReasonNameLength
	}
	if utf8.RuneCountInString(input.Description) > maxDescLen {
		fields["description"] = ReasonTooLong
	}
	if len(input.Subjects) > maxSubjectsPerProgram {
		fields["subjects"] = ReasonLimit
	}

	seenSubjects := make(map[string]bool, len(input.Subjects))
	for i := range input.Subjects {
		subject := &input.Subjects[i]
		subject.Name = strings.TrimSpace(subject.Name)
		path := fmt.Sprintf("subjects[%d]", i)
		key := strings.ToLower(subject.Name)
		if n := utf8.RuneCountInString(subject.Name); n < 1 || n > maxNameLen {
			fields[path+".name"] = ReasonNameLength
		} else if seenSubjects[key] {
			fields[path+".name"] = ReasonDuplicate
		}
		seenSubjects[key] = true

		if len(subject.Chapters) > maxChaptersPerSubject {
			fields[path+".chapters"] = ReasonLimit
		}
		seenChapters := make(map[string]bool, len(subject.Chapters))
		for j := range subject.Chapters {
			chapter := &subject.Chapters[j]
			chapter.Name = strings.TrimSpace(chapter.Name)
			chapterPath := fmt.Sprintf("%s.chapters[%d].name", path, j)
			chapterKey := strings.ToLower(chapter.Name)
			if n := utf8.RuneCountInString(chapter.Name); n < 1 || n > maxNameLen {
				fields[chapterPath] = ReasonNameLength
			} else if seenChapters[chapterKey] {
				fields[chapterPath] = ReasonDuplicate
			}
			seenChapters[chapterKey] = true
		}
	}

	if len(fields) > 0 {
		return ProgramInput{}, &InputError{Fields: fields}
	}
	return input, nil
}

// foreignItems refuses ids this program does not own; a chapter may move between its own subjects.
func foreignItems(current model.ProgramTree, input ProgramInput) error {
	subjects := make(map[uuid.UUID]bool, len(current.Subjects))
	chapters := make(map[uuid.UUID]bool)
	for _, subject := range current.Subjects {
		subjects[subject.Subject.ID] = true
		for _, chapter := range subject.Chapters {
			chapters[chapter.ID] = true
		}
	}

	fields := map[string]string{}
	for i, subject := range input.Subjects {
		if subject.ID != uuid.Nil && !subjects[subject.ID] {
			fields[fmt.Sprintf("subjects[%d].id", i)] = ReasonForeignItem
		}
		for j, chapter := range subject.Chapters {
			if chapter.ID != uuid.Nil && !chapters[chapter.ID] {
				fields[fmt.Sprintf("subjects[%d].chapters[%d].id", i, j)] = ReasonForeignItem
			}
		}
	}
	if len(fields) > 0 {
		return &InputError{Fields: fields}
	}
	return nil
}

func treeOf(input ProgramInput) model.ProgramTree {
	tree := model.ProgramTree{Program: model.Program{
		Code: input.Code, Name: input.Name, Description: input.Description,
	}}
	for si, subject := range input.Subjects {
		out := model.SubjectTree{Subject: model.Subject{ID: subject.ID, Name: subject.Name, Position: si}}
		for ci, chapter := range subject.Chapters {
			out.Chapters = append(out.Chapters, model.Chapter{ID: chapter.ID, Name: chapter.Name, Position: ci})
		}
		tree.Subjects = append(tree.Subjects, out)
	}
	return tree
}

// programError turns the repository's causes into the answers the API promises; anything else is a
// genuine failure and passes through to the error handler.
func programError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrNotFound):
		return ErrProgramNotFound
	case errors.Is(err, repository.ErrDuplicateCode):
		return ErrProgramCodeTaken
	case errors.Is(err, repository.ErrDuplicate):
		return ErrDuplicateName
	case errors.Is(err, repository.ErrReferencedChapters):
		return ErrChapterReferenced
	case errors.Is(err, repository.ErrReferencedSubjects):
		return ErrSubjectReferenced
	case errors.Is(err, repository.ErrReferenced):
		return ErrProgramReferenced
	}
	return err
}

func (s *CurriculumService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
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
