package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

func TestDashboardSummaryEmptyAndRetakes(t *testing.T) {
	empty := summarizeDashboard(0, "0.00", nil)
	if empty.OwnedTryouts != 0 || empty.CompletedTryouts != 0 || empty.AverageScore != nil || empty.Balance != "0.00" {
		t.Fatalf("empty summary = %+v", empty)
	}
	first, second := uuid.New(), uuid.New()
	zero, high, middle := 0.0, 100.0, 66.67
	out := summarizeDashboard(3, "1234567890123.45", []model.AttemptSummary{
		{TryoutID: first, Status: model.SessionCompleted, Score: &zero},
		{TryoutID: first, Status: model.SessionCompleted, Score: &high},
		{TryoutID: second, Status: model.SessionCompleted, Score: &middle},
		{TryoutID: uuid.New(), Status: model.SessionInProgress},
	})
	if out.OwnedTryouts != 3 || out.CompletedTryouts != 2 || out.AverageScore == nil || *out.AverageScore != 55.56 {
		t.Fatalf("retake summary = %+v", out)
	}
	if out.Balance != "1234567890123.45" {
		t.Fatalf("balance lost precision: %s", out.Balance)
	}
	zeroOnly := summarizeDashboard(1, "0.00", []model.AttemptSummary{{TryoutID: first, Status: model.SessionCompleted, Score: &zero}})
	if zeroOnly.AverageScore == nil || *zeroOnly.AverageScore != 0 {
		t.Fatalf("zero score must be present: %+v", zeroOnly)
	}
}
