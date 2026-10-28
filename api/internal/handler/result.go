package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

const (
	explanationsNotReleased = "Pembahasan belum dibuka."
	resultQueryRule         = "Nilai tidak valid. Limit 1–100, offset 0 atau lebih, pencarian maksimal 100 karakter."
)

const codeExplanationsNotReleased httpx.Code = "explanations_not_released"

type rankingResponse struct {
	Rank             int     `json:"rank"`
	ParticipantCount int     `json:"participant_count"`
	AverageScore     float64 `json:"average_score"`
	AttemptsCounted  int     `json:"attempts_counted"`
}

// resultDetailResponse is a result with the live rank. It never carries the IRT warning: that is staff-only.
type resultDetailResponse struct {
	resultResponse
	Ranking              rankingResponse `json:"ranking"`
	ExplanationsReleased bool            `json:"explanations_released"`
	ExplanationReleaseAt *time.Time      `json:"explanation_release_at"`
}

type reviewOptionResponse struct {
	ID     string   `json:"id"`
	Label  string   `json:"label"`
	Points *float64 `json:"points"`
}

type reviewQuestionResponse struct {
	ID             uuid.UUID              `json:"id"`
	Position       int                    `json:"position"`
	Type           string                 `json:"type"`
	Content        string                 `json:"content"`
	Options        []reviewOptionResponse `json:"options"`
	OptionWeighted bool                   `json:"option_weighted"`
	Answer         []string               `json:"answer"`
	Correct        []string               `json:"correct"`
	IsCorrect      bool                   `json:"is_correct"`
	Explanation    string                 `json:"explanation"`
}

type reviewSubtestResponse struct {
	SubtestID     uuid.UUID                `json:"subtest_id"`
	Name          string                   `json:"name"`
	Position      int                      `json:"position"`
	Score         float64                  `json:"score"`
	CorrectCount  int                      `json:"correct_count"`
	QuestionCount int                      `json:"question_count"`
	Questions     []reviewQuestionResponse `json:"questions"`
}

type reviewResponse struct {
	SessionID uuid.UUID               `json:"session_id"`
	TryoutID  uuid.UUID               `json:"tryout_id"`
	Subtests  []reviewSubtestResponse `json:"subtests"`
}

type leaderboardEntryResponse struct {
	Rank            int       `json:"rank"`
	UserID          uuid.UUID `json:"user_id"`
	StudentName     string    `json:"student_name"`
	AverageScore    float64   `json:"average_score"`
	AttemptsCounted int       `json:"attempts_counted"`
}

type leaderboardResponse struct {
	Entries []leaderboardEntryResponse `json:"entries"`
	Total   int                        `json:"total"`
}

type staffResultResponse struct {
	SessionID        uuid.UUID               `json:"session_id"`
	TryoutID         uuid.UUID               `json:"tryout_id"`
	TryoutTitle      string                  `json:"tryout_title"`
	ProgramCode      string                  `json:"program_code"`
	ScoringMode      string                  `json:"scoring_mode"`
	UserID           uuid.UUID               `json:"user_id"`
	StudentName      string                  `json:"student_name"`
	AttemptNumber    int                     `json:"attempt_number"`
	Score            float64                 `json:"score"`
	AverageScore     float64                 `json:"average_score"`
	Rank             int                     `json:"rank"`
	ParticipantCount int                     `json:"participant_count"`
	IRTWarning       bool                    `json:"irt_warning"`
	CompletedAt      time.Time               `json:"completed_at"`
	Subtests         []resultSubtestResponse `json:"subtests"`
}

type staffResultListResponse struct {
	Results []staffResultResponse `json:"results"`
	Total   int                   `json:"total"`
}

func (h *Handler) TryoutSessionResult(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	detail, err := h.results.Detail(c.Request().Context(), scope, principal, id)
	if err != nil {
		return resultError(err)
	}
	return c.JSON(http.StatusOK, resultDetailResponse{
		resultResponse: resultResponseOf(detail.Result),
		Ranking: rankingResponse{
			Rank: detail.Ranking.Rank, ParticipantCount: detail.Ranking.Participants,
			AverageScore: detail.Ranking.Average, AttemptsCounted: detail.Ranking.Attempts,
		},
		ExplanationsReleased: detail.Released, ExplanationReleaseAt: detail.ReleaseAt,
	})
}

func (h *Handler) TryoutSessionReview(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := runtimeID(c, "id")
	if err != nil {
		return err
	}
	review, err := h.results.Review(c.Request().Context(), scope, principal, id)
	if err != nil {
		return resultError(err)
	}
	body := reviewResponse{SessionID: review.Result.SessionID, TryoutID: review.Result.TryoutID,
		Subtests: make([]reviewSubtestResponse, 0, len(review.Subtests))}
	for _, st := range review.Subtests {
		out := reviewSubtestResponse{
			SubtestID: st.Part.SubtestID, Name: st.Part.Name, Position: st.Part.Position, Score: st.Part.Score,
			CorrectCount: st.Part.CorrectCount, QuestionCount: st.Part.QuestionCount,
			Questions: make([]reviewQuestionResponse, 0, len(st.Questions)),
		}
		for _, item := range st.Questions {
			q := item.Question
			rq := reviewQuestionResponse{
				ID: q.QuestionID, Position: q.Position, Type: q.Type, Content: q.Content,
				Options: make([]reviewOptionResponse, 0, len(q.Options)), OptionWeighted: q.OptionWeighted,
				Answer: item.Answer, Correct: []string{}, IsCorrect: item.Correct, Explanation: q.Explanation,
			}
			for _, o := range q.Options {
				ro := reviewOptionResponse{ID: o.ID, Label: o.Label}
				if q.OptionWeighted {
					points := 1.0
					if o.Points != nil {
						points = *o.Points
					}
					ro.Points = &points
				}
				rq.Options = append(rq.Options, ro)
			}
			if !q.OptionWeighted && q.Correct != nil {
				rq.Correct = q.Correct
			}
			out.Questions = append(out.Questions, rq)
		}
		body.Subtests = append(body.Subtests, out)
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) TryoutLeaderboard(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return httpx.NotFound(tryoutNotFound)
	}
	limit, offset, err := orderPageOf(c)
	if err != nil {
		return err
	}
	standings, total, err := h.results.Leaderboard(c.Request().Context(), scope, principal, id, limit, offset)
	if err != nil {
		return resultError(err)
	}
	body := leaderboardResponse{Entries: make([]leaderboardEntryResponse, 0, len(standings)), Total: total}
	for _, s := range standings {
		body.Entries = append(body.Entries, leaderboardEntryResponse{
			Rank: s.Rank, UserID: s.UserID, StudentName: s.Name, AverageScore: s.Average, AttemptsCounted: s.Attempts,
		})
	}
	return c.JSON(http.StatusOK, body)
}

func (h *Handler) StaffResults(c echo.Context) error {
	scope, principal, ok := runtimeCaller(c)
	if !ok {
		return httpx.Unauthorized(sessionInvalid)
	}
	limit, offset, err := orderPageOf(c)
	if err != nil {
		return err
	}
	filter := service.StaffResultFilter{Search: c.QueryParam("q"), Limit: limit, Offset: offset}
	if raw := c.QueryParam("tryout_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return httpx.Validation(incomplete, map[string]string{"tryout_id": invalidFormat})
		}
		filter.TryoutID = &id
	}
	rows, total, err := h.results.StaffResults(c.Request().Context(), scope, principal, filter)
	if err != nil {
		return resultError(err)
	}
	body := staffResultListResponse{Results: make([]staffResultResponse, 0, len(rows)), Total: total}
	for _, r := range rows {
		item := staffResultResponse{
			SessionID: r.SessionID, TryoutID: r.TryoutID, TryoutTitle: r.TryoutTitle, ProgramCode: r.ProgramCode,
			ScoringMode: r.ScoringMode, UserID: r.UserID, StudentName: r.StudentName, AttemptNumber: r.AttemptNumber,
			Score: r.Score, AverageScore: r.Average, Rank: r.Rank, ParticipantCount: r.Participants,
			IRTWarning: r.IRTWarning, CompletedAt: r.CompletedAt, Subtests: make([]resultSubtestResponse, 0, len(r.Subtests)),
		}
		for _, p := range r.Subtests {
			item.Subtests = append(item.Subtests, partResultResponseOf(p))
		}
		body.Results = append(body.Results, item)
	}
	return c.JSON(http.StatusOK, body)
}

func partResultResponseOf(p model.PartResult) resultSubtestResponse {
	return resultSubtestResponse{
		SubtestID: p.SubtestID, Name: p.Name, Position: p.Position, Score: p.Score,
		CorrectCount: p.CorrectCount, QuestionCount: p.QuestionCount,
	}
}

func resultError(err error) error {
	if errors.Is(err, service.ErrExplanationsNotReleased) {
		return httpx.New(http.StatusConflict, codeExplanationsNotReleased, explanationsNotReleased)
	}
	return runtimeError(err)
}
