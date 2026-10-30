package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	programNotFound      = "Program tidak ditemukan."
	programCodeTaken     = "Kode program sudah dipakai di tenant ini."
	programDuplicateName = "Nama sudah dipakai di program ini."
	programReferenced    = "Program masih dipakai konten lain."
	chapterReferenced    = "Bab materi program ini masih dipakai konten lain."
	subjectReferenced    = "Mata pelajaran program ini masih dipakai konten lain."
	codeRule             = "Kode 2–12 karakter: huruf, angka, _ atau -."
	nameRule             = "Nama 1–100 karakter."
	descriptionRule      = "Deskripsi maksimal 500 karakter."
	duplicateNameRule    = "Nama tidak boleh sama (tanpa membedakan huruf besar-kecil)."
	limitRule            = "Jumlah melebihi batas."
	foreignItemRule      = "Item ini bukan milik program tersebut."
)

type programResponse struct {
	ID          uuid.UUID         `json:"id"`
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Subjects    []subjectResponse `json:"subjects"`
}

type subjectResponse struct {
	ID       uuid.UUID         `json:"id"`
	Name     string            `json:"name"`
	Chapters []chapterResponse `json:"chapters"`
}

type chapterResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type programsResponse struct {
	Programs []programResponse `json:"programs"`
}

type publicProgramResponse struct {
	ID          uuid.UUID `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
}

type publicProgramsResponse struct {
	Programs []publicProgramResponse `json:"programs"`
}

type createProgramRequest struct {
	Code        string                 `json:"code"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Subjects    []createSubjectRequest `json:"subjects"`
}

type createSubjectRequest struct {
	Name     string                 `json:"name"`
	Chapters []createChapterRequest `json:"chapters"`
}

type createChapterRequest struct {
	Name string `json:"name"`
}

// The replace body may carry ids; an omitted id creates the item.
type updateProgramRequest struct {
	Code        string                 `json:"code"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Subjects    []updateSubjectRequest `json:"subjects"`
}

type updateSubjectRequest struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Chapters []updateChapterRequest `json:"chapters"`
}

type updateChapterRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (h *Handler) ListPrograms(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	trees, err := h.curriculum.List(c.Request().Context(), scope, principal)
	if err != nil {
		return curriculumError(err)
	}
	body := programsResponse{Programs: make([]programResponse, 0, len(trees))}
	for _, tree := range trees {
		body.Programs = append(body.Programs, programResponseOf(tree))
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) CatalogPrograms(c echo.Context) error {
	scope, ok := hostScope(c)
	if !ok {
		return httpx.NotFound(productCatalogMissing)
	}
	programs, err := h.curriculum.PublicPrograms(c.Request().Context(), scope)
	if err != nil {
		return curriculumError(err)
	}
	body := publicProgramsResponse{Programs: make([]publicProgramResponse, 0, len(programs))}
	for _, p := range programs {
		body.Programs = append(body.Programs, publicProgramResponse{ID: p.ID, Code: p.Code, Name: p.Name, Description: p.Description})
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) CreateProgram(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	var req createProgramRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	input := service.ProgramInput{Code: req.Code, Name: req.Name, Description: req.Description}
	for _, subject := range req.Subjects {
		in := service.SubjectInput{Name: subject.Name}
		for _, chapter := range subject.Chapters {
			in.Chapters = append(in.Chapters, service.ChapterInput{Name: chapter.Name})
		}
		input.Subjects = append(input.Subjects, in)
	}
	tree, err := h.curriculum.Create(c.Request().Context(), scope, principal, input)
	if err != nil {
		return curriculumError(err)
	}
	return c.JSON(http.StatusCreated, programResponseOf(tree))
}

func (h *Handler) ReplaceProgram(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(programNotFound)
	}
	var req updateProgramRequest
	if err := bindStrict(c, &req); err != nil {
		return err
	}
	input, err := programInputOf(req)
	if err != nil {
		return err
	}
	tree, err := h.curriculum.Replace(c.Request().Context(), scope, principal, id, input)
	if err != nil {
		return curriculumError(err)
	}
	return c.JSON(http.StatusOK, programResponseOf(tree))
}

func (h *Handler) DeleteProgram(c echo.Context) error {
	scope, principal, ok := curriculumCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(programNotFound)
	}
	if err := h.curriculum.Delete(c.Request().Context(), scope, principal, id); err != nil {
		return curriculumError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func curriculumCaller(c echo.Context) (service.HostScope, model.Principal, bool) {
	scope, scoped := hostScope(c)
	principal, signedIn := PrincipalFrom(c)
	return scope, principal, scoped && signedIn
}

// programInputOf turns the wire body into the service's input, naming bad ids by their path.
func programInputOf(req updateProgramRequest) (service.ProgramInput, error) {
	fields := map[string]string{}
	input := service.ProgramInput{Code: req.Code, Name: req.Name, Description: req.Description}
	for i, subject := range req.Subjects {
		in := service.SubjectInput{Name: subject.Name}
		if subject.ID != "" {
			id, err := uuid.Parse(subject.ID)
			if err != nil {
				fields[fmt.Sprintf("subjects[%d].id", i)] = invalidFormat
			} else {
				in.ID = id
			}
		}
		for j, chapter := range subject.Chapters {
			out := service.ChapterInput{Name: chapter.Name}
			if chapter.ID != "" {
				id, err := uuid.Parse(chapter.ID)
				if err != nil {
					fields[fmt.Sprintf("subjects[%d].chapters[%d].id", i, j)] = invalidFormat
				} else {
					out.ID = id
				}
			}
			in.Chapters = append(in.Chapters, out)
		}
		input.Subjects = append(input.Subjects, in)
	}
	if len(fields) > 0 {
		return service.ProgramInput{}, httpx.Validation(incomplete, fields)
	}
	return input, nil
}

func programResponseOf(tree model.ProgramTree) programResponse {
	out := programResponse{
		ID: tree.Program.ID, Code: tree.Program.Code, Name: tree.Program.Name,
		Description: tree.Program.Description,
		Subjects:    make([]subjectResponse, 0, len(tree.Subjects)),
	}
	for _, subject := range tree.Subjects {
		subjectOut := subjectResponse{
			ID: subject.Subject.ID, Name: subject.Subject.Name,
			Chapters: make([]chapterResponse, 0, len(subject.Chapters)),
		}
		for _, chapter := range subject.Chapters {
			subjectOut.Chapters = append(subjectOut.Chapters, chapterResponse{ID: chapter.ID, Name: chapter.Name})
		}
		out.Subjects = append(out.Subjects, subjectOut)
	}
	return out
}

func curriculumError(err error) error {
	var input *service.InputError
	if errors.As(err, &input) {
		fields := make(map[string]string, len(input.Fields))
		for path, reason := range input.Fields {
			fields[path] = curriculumReason(reason)
		}
		return httpx.Validation(incomplete, fields)
	}
	switch {
	case errors.Is(err, service.ErrProgramNotFound):
		return httpx.NotFound(programNotFound)
	case errors.Is(err, service.ErrTenantNotFound):
		return httpx.NotFound(tenantNotFound)
	case errors.Is(err, service.ErrProgramCodeTaken):
		return httpx.Conflict(programCodeTaken).WithFields(map[string]string{"code": programCodeTaken})
	case errors.Is(err, service.ErrDuplicateName):
		return httpx.Conflict(programDuplicateName)
	case errors.Is(err, service.ErrChapterReferenced):
		return httpx.Conflict(chapterReferenced)
	case errors.Is(err, service.ErrSubjectReferenced):
		return httpx.Conflict(subjectReferenced)
	case errors.Is(err, service.ErrProgramReferenced):
		return httpx.Conflict(programReferenced)
	}
	return err
}

func curriculumReason(reason string) string {
	switch reason {
	case service.ReasonCode:
		return codeRule
	case service.ReasonNameLength:
		return nameRule
	case service.ReasonTooLong:
		return descriptionRule
	case service.ReasonDuplicate:
		return duplicateNameRule
	case service.ReasonLimit:
		return limitRule
	case service.ReasonForeignItem:
		return foreignItemRule
	}
	return invalidFormat
}
