package service

import (
	"context"
	"errors"
	"time"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var ErrReportForbidden = errors.New("revenue recap is Admin only")

const (
	ReasonReportRange = "report_range"
	reportDateLayout  = "2006-01-02"
	maxReportDays     = 366
	defaultReportDays = 30
)

// wib is Asia/Jakarta; it has no daylight saving, so a fixed zone agrees with PostgreSQL's day grouping
// without needing tzdata in the runtime image.
var wib = time.FixedZone("Asia/Jakarta", 7*60*60)

type ReportService struct {
	repo *repository.ReportRepository
	now  func() time.Time
}

func NewReportService(repo *repository.ReportRepository) *ReportService {
	return &ReportService{repo: repo, now: time.Now}
}

// WithClock replaces the clock the default range is measured from; tests only.
func (s *ReportService) WithClock(now func() time.Time) *ReportService {
	s.now = now
	return s
}

type RevenueInput struct {
	// From and To are inclusive Asia/Jakarta days as YYYY-MM-DD; both empty means the last 30 days.
	From, To string
	Limit    int
	Offset   int
}

// Revenue is the tenant Admin's recap. It reports what was confirmed and approved; nothing here settles money.
func (s *ReportService) Revenue(ctx context.Context, scope HostScope, principal model.Principal, in RevenueInput) (model.RevenueRecap, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.RevenueRecap{}, err
	}
	if principal.Role != model.RoleAdmin {
		return model.RevenueRecap{}, ErrReportForbidden
	}
	from, to, err := s.reportRange(in)
	if err != nil {
		return model.RevenueRecap{}, err
	}
	limit, offset, err := checkedWalletPage(in.Limit, in.Offset)
	if err != nil {
		return model.RevenueRecap{}, err
	}
	recap, err := s.repo.Revenue(ctx, tenantID, from, to.AddDate(0, 0, 1), limit, offset)
	if err != nil {
		return model.RevenueRecap{}, err
	}
	recap.From, recap.To = from, to
	return recap, nil
}

func (s *ReportService) reportRange(in RevenueInput) (time.Time, time.Time, error) {
	bad := func(field string) (time.Time, time.Time, error) {
		return time.Time{}, time.Time{}, &InputError{Fields: map[string]string{field: ReasonReportRange}}
	}
	if in.From == "" && in.To == "" {
		today := s.now().In(wib)
		to := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, wib)
		return to.AddDate(0, 0, -(defaultReportDays - 1)), to, nil
	}
	from, err := time.ParseInLocation(reportDateLayout, in.From, wib)
	if err != nil {
		return bad("from")
	}
	to, err := time.ParseInLocation(reportDateLayout, in.To, wib)
	if err != nil {
		return bad("to")
	}
	if to.Before(from) || to.Sub(from) >= maxReportDays*24*time.Hour {
		return bad("to")
	}
	return from, to, nil
}
