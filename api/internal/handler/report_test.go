package handler

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
)

func (f userOpsFixture) insertOrder(t *testing.T, tenantID, userID uuid.UUID, productType, method, status, price, confirmedAt string) {
	t.Helper()
	var confirmed any
	if confirmedAt != "" {
		confirmed = confirmedAt
	}
	cancelled, reason := any(nil), ""
	if status == "cancelled" {
		cancelled, reason = confirmedAt, "uji"
		confirmed = nil
	}
	if _, err := f.pool.Exec(context.Background(), `
		INSERT INTO orders (tenant_id, user_id, product_id, product_type, product_title, price, payment_method, status,
		                    idempotency_key, confirmed_at, cancelled_at, cancel_reason)
		VALUES ($1, $2, gen_random_uuid(), $3, 'Produk uji', $4, $5, $6, $7, $8::timestamptz, $9::timestamptz, $10)`,
		tenantID, userID, productType, price, method, status, "rev-"+uuid.NewString()[:12], confirmed, cancelled, reason); err != nil {
		t.Fatalf("insert order: %v", err)
	}
}

func (f userOpsFixture) insertTopup(t *testing.T, tenantID, userID uuid.UUID, amount, status, at string) {
	t.Helper()
	var approved any
	if status == "approved" {
		approved = at
	}
	if _, err := f.pool.Exec(context.Background(), `
		INSERT INTO topup_requests (tenant_id, user_id, amount, method, status, idempotency_key, approved_at, rejected_at, reject_reason)
		VALUES ($1, $2, $3, 'gateway', $4, $5, $6::timestamptz, $7::timestamptz, $8)`,
		tenantID, userID, amount, status, "rev-"+uuid.NewString()[:12], approved,
		map[bool]any{true: at, false: nil}[status == "rejected"], map[bool]string{true: "uji", false: ""}[status == "rejected"]); err != nil {
		t.Fatalf("insert topup: %v", err)
	}
}

func TestRevenueRecapSumsExactlyInJakartaDays(t *testing.T) {
	f := newUserOpsFixture(t)
	a, s := f.tenantA.ID, f.secondStudent.ID

	// 16:59:59Z is still 1 March in Jakarta and 17:00:00Z is already 2 March.
	f.insertOrder(t, a, s, "individual", "wallet", "confirmed", "100000.10", "2027-03-01 16:59:59+00")
	f.insertOrder(t, a, s, "bundle", "gateway", "confirmed", "250000.25", "2027-03-01 00:00:00+00")
	f.insertOrder(t, a, s, "individual", "wallet", "confirmed", "70000.00", "2027-03-01 17:00:00+00")
	f.insertOrder(t, a, s, "individual", "wallet", "confirmed", "999.00", "2027-02-28 16:59:59+00")
	f.insertOrder(t, a, s, "individual", "wallet", "pending", "55555.00", "")
	f.insertOrder(t, a, s, "individual", "wallet", "cancelled", "44444.00", "2027-03-01 03:00:00+00")
	f.insertOrder(t, f.tenantB.ID, f.studentB.ID, "individual", "wallet", "confirmed", "88888.00", "2027-03-01 03:00:00+00")
	f.insertTopup(t, a, s, "30000", "approved", "2027-03-01 10:00:00+00")
	f.insertTopup(t, a, s, "20000", "approved", "2027-03-01 16:00:00+00")
	f.insertTopup(t, a, s, "10000", "rejected", "2027-03-01 10:00:00+00")
	f.insertTopup(t, a, s, "77000", "approved", "2027-03-03 10:00:00+00")
	f.insertTopup(t, f.tenantB.ID, f.studentB.ID, "66000", "approved", "2027-03-01 10:00:00+00")

	recap := func(query url.Values, want int) revenueRecapResponse {
		t.Helper()
		rec := f.call(http.MethodGet, "/api/v1/admin/reports/revenue?"+query.Encode(), nil, f.tokenAdminA)
		expect(t, "recap "+query.Encode(), rec, want)
		if want != http.StatusOK {
			return revenueRecapResponse{}
		}
		return decodeAs[revenueRecapResponse](t, rec)
	}

	day := recap(url.Values{"from": {"2027-03-01"}, "to": {"2027-03-01"}}, http.StatusOK)
	if day.Orders.Count != 2 || day.Orders.Total != "350000.35" {
		t.Errorf("orders = %+v, want the two orders of 1 March (350000.35)", day.Orders)
	}
	if day.TopupsApproved.Count != 2 || day.TopupsApproved.Total != "50000.00" {
		t.Errorf("topups = %+v, want the two approved top-ups (50000.00)", day.TopupsApproved)
	}
	wantMethod := map[string]revenueBucketResponse{"gateway": {Key: "gateway", Count: 1, Total: "250000.25"}, "wallet": {Key: "wallet", Count: 1, Total: "100000.10"}}
	wantProduct := map[string]revenueBucketResponse{"bundle": {Key: "bundle", Count: 1, Total: "250000.25"}, "individual": {Key: "individual", Count: 1, Total: "100000.10"}}
	for name, c := range map[string]struct {
		got  []revenueBucketResponse
		want map[string]revenueBucketResponse
	}{"method": {day.ByPaymentMethod, wantMethod}, "product": {day.ByProductType, wantProduct}} {
		if len(c.got) != len(c.want) {
			t.Errorf("by %s = %+v, want %+v", name, c.got, c.want)
		}
		for _, b := range c.got {
			if c.want[b.Key] != b {
				t.Errorf("by %s bucket = %+v, want %+v", name, b, c.want[b.Key])
			}
		}
	}
	if day.DaysTotal != 1 || len(day.Days) != 1 || day.Days[0].Date != "2027-03-01" || day.Days[0].OrdersTotal != "350000.35" || day.Days[0].TopupsTotal != "50000.00" {
		t.Errorf("days = %+v, want the single day 2027-03-01", day.Days)
	}

	week := recap(url.Values{"from": {"2027-02-28"}, "to": {"2027-03-03"}}, http.StatusOK)
	if week.Orders.Count != 4 || week.Orders.Total != "420999.35" || week.TopupsApproved.Count != 3 || week.TopupsApproved.Total != "127000.00" {
		t.Errorf("week = orders %+v topups %+v, want 4 orders (420999.35) and 3 top-ups (127000.00)", week.Orders, week.TopupsApproved)
	}
	if week.DaysTotal != 4 || len(week.Days) != 4 || week.Days[0].Date != "2027-03-03" || week.Days[3].Date != "2027-02-28" {
		t.Errorf("week days = %+v, want four days newest first", week.Days)
	}
	paged := recap(url.Values{"from": {"2027-02-28"}, "to": {"2027-03-03"}, "limit": {"2"}, "offset": {"1"}}, http.StatusOK)
	if paged.DaysTotal != 4 || len(paged.Days) != 2 || paged.Days[0].Date != "2027-03-02" || paged.Days[1].Date != "2027-03-01" {
		t.Errorf("paged days = %+v, want 2027-03-02 then 2027-03-01 of 4", paged.Days)
	}
	if paged.Orders.Total != week.Orders.Total {
		t.Errorf("a page changed the totals: %s vs %s", paged.Orders.Total, week.Orders.Total)
	}

	empty := recap(url.Values{"from": {"2030-01-01"}, "to": {"2030-01-02"}}, http.StatusOK)
	if empty.Orders.Count != 0 || empty.Orders.Total != "0.00" || empty.TopupsApproved.Total != "0.00" || len(empty.Days) != 0 || empty.ByPaymentMethod == nil {
		t.Errorf("empty = %+v, want zeros and empty lists", empty)
	}
	if def := recap(url.Values{}, http.StatusOK); def.From == "" || def.To == "" {
		t.Errorf("default range = %s..%s, want a window", def.From, def.To)
	}
	for name, q := range map[string]url.Values{
		"bad date":    {"from": {"01-03-2027"}, "to": {"2027-03-02"}},
		"reversed":    {"from": {"2027-03-02"}, "to": {"2027-03-01"}},
		"too long":    {"from": {"2025-01-01"}, "to": {"2027-03-01"}},
		"only a from": {"from": {"2027-03-01"}},
		"bad limit":   {"limit": {"101"}, "from": {"2027-03-01"}, "to": {"2027-03-01"}},
	} {
		recap(q, http.StatusUnprocessableEntity)
		_ = name
	}
}
