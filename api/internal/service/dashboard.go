package service

import (
	"context"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

type DashboardSummary struct {
	OwnedTryouts     int
	CompletedTryouts int
	AverageScore     *float64
	Balance          string
}

func summarizeDashboard(owned int, balance string, attempts []model.AttemptSummary) DashboardSummary {
	out := DashboardSummary{OwnedTryouts: owned, Balance: balance}
	completed := make(map[uuid.UUID]bool)
	var sum float64
	var count int
	for _, attempt := range attempts {
		if attempt.Status != model.SessionCompleted || attempt.Score == nil {
			continue
		}
		completed[attempt.TryoutID] = true
		sum += *attempt.Score
		count++
	}
	out.CompletedTryouts = len(completed)
	if count > 0 {
		average := math.Round(sum/float64(count)*100) / 100
		out.AverageScore = &average
	}
	return out
}

func (s *RuntimeService) Dashboard(ctx context.Context, scope HostScope, principal model.Principal) (DashboardSummary, error) {
	tenantID, err := studentOf(scope, principal)
	if err != nil {
		return DashboardSummary{}, err
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return DashboardSummary{}, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	owned, balance, err := s.repo.DashboardAccount(ctx, tx, tenantID, principal.UserID)
	if err != nil {
		return DashboardSummary{}, runtimeError(err)
	}
	attempts, err := s.repo.Attempts(ctx, tx, tenantID, principal.UserID, nil)
	if err != nil {
		return DashboardSummary{}, runtimeError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return DashboardSummary{}, err
	}
	return summarizeDashboard(owned, balance, attempts), nil
}
