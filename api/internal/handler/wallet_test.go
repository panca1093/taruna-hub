package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/httpx"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/service"
)

type walletHTTPFixture struct {
	authFixture
	tenantA, tenantB          model.Tenant
	hostA, hostB              string
	studentA, secondStudent   model.User
	studentB                  model.User
	adminA, operatorA         model.User
	adminB                    model.User
	tokenA, tokenSecond       string
	tokenB, tokenAdminA       string
	tokenOperatorA, tokenAdmB string
}

func newWalletHTTPFixture(t *testing.T) walletHTTPFixture {
	t.Helper()
	f := walletHTTPFixture{authFixture: newAuthFixture(t, "localhost", false)}
	f.tenantA, f.tenantB = f.active[0], f.active[1]
	f.hostA, f.hostB = f.host(f.tenantA.Slug), f.host(f.tenantB.Slug)
	// Registered after the tenant cleanup, so the ledger rows go before their users are deleted.
	t.Cleanup(func() { wipeWalletEntries(t, f.pool, []uuid.UUID{f.tenantA.ID, f.tenantB.ID}) })

	f.studentA, f.tokenA = walletAccount(t, f.authFixture, f.tenantA, model.RoleStudent)
	f.secondStudent, f.tokenSecond = walletAccount(t, f.authFixture, f.tenantA, model.RoleStudent)
	f.studentB, f.tokenB = walletAccount(t, f.authFixture, f.tenantB, model.RoleStudent)
	f.adminA, f.tokenAdminA = walletAccount(t, f.authFixture, f.tenantA, model.RoleAdmin)
	f.operatorA, f.tokenOperatorA = walletAccount(t, f.authFixture, f.tenantA, model.RoleOperator)
	f.adminB, f.tokenAdmB = walletAccount(t, f.authFixture, f.tenantB, model.RoleAdmin)
	return f
}

// walletAccount creates one account of the fixture tenant and signs it in through the real service.
func walletAccount(t *testing.T, f authFixture, tenant model.Tenant, role model.Role) (model.User, string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(authTestPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	email := "wallet-" + uuid.NewString()[:8] + "@example.com"
	user, err := repository.NewUserRepository(f.pool).Create(context.Background(), model.User{
		TenantID: &tenant.ID, Name: "Wallet " + string(role), Email: &email,
		PasswordHash: string(hash), Role: role,
	})
	if err != nil {
		t.Fatalf("create %s account: %v", role, err)
	}
	token := loginToken(t, newAuthService(f.tenantFixture), service.HostScope{Tenant: tenant}, email, authTestPassword)
	return user, token
}

// wipeWalletEntries switches the trigger off for one cleanup connection only, never for the table.
func wipeWalletEntries(t *testing.T, pool *pgxpool.Pool, tenantIDs []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Errorf("acquire cleanup connection: %v", err)
		return
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SET session_replication_role = replica`); err != nil {
		t.Errorf("disable the ledger trigger for cleanup: %v", err)
		return
	}
	defer func() {
		if _, err := conn.Exec(ctx, `RESET session_replication_role`); err != nil {
			t.Errorf("restore the ledger trigger after cleanup: %v", err)
		}
	}()
	if _, err := conn.Exec(ctx, `DELETE FROM wallet_entries WHERE tenant_id = ANY($1)`, tenantIDs); err != nil {
		t.Errorf("cleanup wallet entries: %v", err)
	}
}

func adjustmentBody(adjustType, amount, reason, key string) map[string]any {
	return map[string]any{"type": adjustType, "amount": amount, "reason": reason, "idempotency_key": key}
}

func decodeWallet(t *testing.T, rec *httptest.ResponseRecorder) walletResponse {
	t.Helper()
	var body walletResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a wallet: %v", rec.Body.String(), err)
	}
	return body
}

func decodeWalletEntries(t *testing.T, rec *httptest.ResponseRecorder) walletEntriesResponse {
	t.Helper()
	var body walletEntriesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a wallet ledger: %v", rec.Body.String(), err)
	}
	return body
}

func decodeWalletStudents(t *testing.T, rec *httptest.ResponseRecorder) walletStudentsResponse {
	t.Helper()
	var body walletStudentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not a student list: %v", rec.Body.String(), err)
	}
	return body
}

func decodeAdjustment(t *testing.T, rec *httptest.ResponseRecorder) adjustmentResponse {
	t.Helper()
	var body adjustmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not an adjustment: %v", rec.Body.String(), err)
	}
	return body
}

func TestWalletIsolatedBetweenTenantsAndUsers(t *testing.T) {
	f := newWalletHTTPFixture(t)
	e := f.echo(t)

	rec := sendTryoutJSON(e, http.MethodPost, f.hostA,
		"/api/v1/students/"+f.studentA.ID.String()+"/wallet/adjustments",
		adjustmentBody("credit", "50000.00", "Hadiah juara kelas", "iso-a-"+uuid.NewString()[:8]), f.tokenAdminA)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin A credits student A: status = %d body = %s", rec.Code, rec.Body.String())
	}
	entryID := decodeAdjustment(t, rec).Entry.ID.String()

	own := decodeWallet(t, getWithToken(e, f.hostA, "/api/v1/wallet", f.tokenA))
	if own.Balance != "50000.00" {
		t.Errorf("student A balance = %s, want 50000.00", own.Balance)
	}
	ownEntries := decodeWalletEntries(t, getWithToken(e, f.hostA, "/api/v1/wallet/entries", f.tokenA))
	if ownEntries.Total != 1 || ownEntries.Entries[0].ID.String() != entryID {
		t.Errorf("student A sees %s, want exactly the adjustment %s", getWithToken(e, f.hostA, "/api/v1/wallet/entries", f.tokenA).Body.String(), entryID)
	}

	// Another student's wallet is not reachable from a student token by any route.
	other := decodeWallet(t, getWithToken(e, f.hostA, "/api/v1/wallet", f.tokenSecond))
	if other.Balance != "0.00" || other.TotalCredits != "0.00" {
		t.Errorf("student B wallet = %+v, want the untouched zero wallet", other)
	}
	if body := getWithToken(e, f.hostA, "/api/v1/wallet/entries", f.tokenSecond).Body.String(); strings.Contains(body, entryID) {
		t.Errorf("student B's ledger carries student A's entry: %s", body)
	}
	for _, path := range []string{
		"/api/v1/students/" + f.studentA.ID.String() + "/wallet",
		"/api/v1/students/" + f.studentA.ID.String() + "/wallet/entries",
	} {
		if rec := getWithToken(e, f.hostA, path, f.tokenSecond); rec.Code != http.StatusForbidden {
			t.Errorf("student B GET %s: status = %d body = %s, want 403", path, rec.Code, rec.Body.String())
		}
	}

	// Tenant B staff cannot read, list or adjust tenant A's student: the same 404 everywhere.
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"wallet":  getWithToken(e, f.hostB, "/api/v1/students/"+f.studentA.ID.String()+"/wallet", f.tokenAdmB),
		"entries": getWithToken(e, f.hostB, "/api/v1/students/"+f.studentA.ID.String()+"/wallet/entries", f.tokenAdmB),
		"adjust": sendTryoutJSON(e, http.MethodPost, f.hostB,
			"/api/v1/students/"+f.studentA.ID.String()+"/wallet/adjustments",
			adjustmentBody("credit", "1000.00", "Silang tenant", "iso-b-"+uuid.NewString()[:8]), f.tokenAdmB),
	} {
		if rec.Code != http.StatusNotFound {
			t.Errorf("tenant B %s on tenant A's student: status = %d body = %s, want 404", name, rec.Code, rec.Body.String())
			continue
		}
		if code := errorEnvelope(t, rec).Error.Code; code != httpx.CodeNotFound {
			t.Errorf("tenant B %s: code = %q, want %q", name, code, httpx.CodeNotFound)
		}
		if strings.Contains(rec.Body.String(), "50000.00") {
			t.Errorf("tenant B %s leaks the other tenant's balance: %s", name, rec.Body.String())
		}
	}
	listB := getWithToken(e, f.hostB, "/api/v1/students", f.tokenAdmB)
	if strings.Contains(listB.Body.String(), f.studentA.ID.String()) {
		t.Errorf("tenant B's student list carries tenant A's student: %s", listB.Body.String())
	}
	for _, student := range decodeWalletStudents(t, listB).Students {
		if student.ID == f.studentA.ID || student.ID == f.secondStudent.ID {
			t.Errorf("tenant B's list carries %s, which belongs to tenant A", student.ID)
		}
	}

	still := decodeWallet(t, getWithToken(e, f.hostA, "/api/v1/wallet", f.tokenA))
	if still.Balance != "50000.00" {
		t.Errorf("after the refused cross-tenant attempts, balance = %s, want 50000.00", still.Balance)
	}
}

func TestWalletRoles(t *testing.T) {
	f := newWalletHTTPFixture(t)
	e := f.echo(t)
	studentPath := "/api/v1/students/" + f.studentA.ID.String()

	cases := []struct {
		method string
		path   string
		body   any
		token  string
		want   int
	}{
		{"GET", "/api/v1/wallet", nil, "", http.StatusUnauthorized},
		{"GET", "/api/v1/wallet", nil, f.tokenA, http.StatusOK},
		{"GET", "/api/v1/wallet", nil, f.tokenOperatorA, http.StatusForbidden},
		{"GET", "/api/v1/wallet", nil, f.tokenAdminA, http.StatusForbidden},
		{"GET", "/api/v1/wallet/entries", nil, "", http.StatusUnauthorized},
		{"GET", "/api/v1/wallet/entries", nil, f.tokenA, http.StatusOK},
		{"GET", "/api/v1/wallet/entries", nil, f.tokenOperatorA, http.StatusForbidden},
		{"GET", "/api/v1/wallet/entries", nil, f.tokenAdminA, http.StatusForbidden},

		{"GET", "/api/v1/students", nil, "", http.StatusUnauthorized},
		{"GET", "/api/v1/students", nil, f.tokenA, http.StatusForbidden},
		{"GET", "/api/v1/students", nil, f.tokenOperatorA, http.StatusOK},
		{"GET", "/api/v1/students", nil, f.tokenAdminA, http.StatusOK},

		{"GET", studentPath + "/wallet", nil, "", http.StatusUnauthorized},
		{"GET", studentPath + "/wallet", nil, f.tokenA, http.StatusForbidden},
		{"GET", studentPath + "/wallet", nil, f.tokenOperatorA, http.StatusOK},
		{"GET", studentPath + "/wallet", nil, f.tokenAdminA, http.StatusOK},
		{"GET", studentPath + "/wallet/entries", nil, "", http.StatusUnauthorized},
		{"GET", studentPath + "/wallet/entries", nil, f.tokenA, http.StatusForbidden},
		{"GET", studentPath + "/wallet/entries", nil, f.tokenOperatorA, http.StatusOK},
		{"GET", studentPath + "/wallet/entries", nil, f.tokenAdminA, http.StatusOK},

		{"POST", studentPath + "/wallet/adjustments", adjustmentBody("credit", "1000.00", "Hadiah", "role-a-"+uuid.NewString()[:8]), "", http.StatusUnauthorized},
		{"POST", studentPath + "/wallet/adjustments", adjustmentBody("credit", "1000.00", "Hadiah", "role-s-"+uuid.NewString()[:8]), f.tokenA, http.StatusForbidden},
		{"POST", studentPath + "/wallet/adjustments", adjustmentBody("credit", "1000.00", "Hadiah", "role-o-"+uuid.NewString()[:8]), f.tokenOperatorA, http.StatusCreated},
		{"POST", studentPath + "/wallet/adjustments", adjustmentBody("credit", "1000.00", "Hadiah", "role-d-"+uuid.NewString()[:8]), f.tokenAdminA, http.StatusCreated},
	}
	for _, tc := range cases {
		rec := sendTryoutJSON(e, tc.method, f.hostA, tc.path, tc.body, tc.token)
		if rec.Code != tc.want {
			t.Errorf("%s %s with token %q: status = %d body = %s, want %d",
				tc.method, tc.path, tc.token, rec.Code, rec.Body.String(), tc.want)
		}
	}

	// The operator's credit and the admin's credit both landed.
	if wallet := decodeWallet(t, getWithToken(e, f.hostA, "/api/v1/wallet", f.tokenA)); wallet.Balance != "2000.00" {
		t.Errorf("balance = %s, want the operator's and the admin's 1000.00", wallet.Balance)
	}
}

func TestWalletResponsesAreNoStore(t *testing.T) {
	f := newWalletHTTPFixture(t)
	e := f.echo(t)
	studentPath := "/api/v1/students/" + f.studentA.ID.String()

	cases := map[string]*httptest.ResponseRecorder{
		"wallet":                    getWithToken(e, f.hostA, "/api/v1/wallet", f.tokenA),
		"wallet without a token":    getWithToken(e, f.hostA, "/api/v1/wallet", ""),
		"entries":                   getWithToken(e, f.hostA, "/api/v1/wallet/entries", f.tokenA),
		"entries with a bad limit":  getWithToken(e, f.hostA, "/api/v1/wallet/entries?limit=abc", f.tokenA),
		"students":                  getWithToken(e, f.hostA, "/api/v1/students", f.tokenAdminA),
		"students as a student":     getWithToken(e, f.hostA, "/api/v1/students", f.tokenA),
		"student wallet":            getWithToken(e, f.hostA, studentPath+"/wallet", f.tokenOperatorA),
		"student wallet of nobody":  getWithToken(e, f.hostA, "/api/v1/students/"+uuid.NewString()+"/wallet", f.tokenAdminA),
		"student entries":           getWithToken(e, f.hostA, studentPath+"/wallet/entries", f.tokenAdminA),
		"adjustment":                sendTryoutJSON(e, http.MethodPost, f.hostA, studentPath+"/wallet/adjustments", adjustmentBody("credit", "5000.00", "Hadiah", "ns-a-"+uuid.NewString()[:8]), f.tokenAdminA),
		"adjustment rejected":       sendTryoutJSON(e, http.MethodPost, f.hostA, studentPath+"/wallet/adjustments", adjustmentBody("credit", "1e3", "Hadiah", "ns-r-"+uuid.NewString()[:8]), f.tokenAdminA),
		"adjustment as an operator": sendTryoutJSON(e, http.MethodPost, f.hostA, studentPath+"/wallet/adjustments", adjustmentBody("credit", "5000.00", "Hadiah", "ns-o-"+uuid.NewString()[:8]), f.tokenOperatorA),
	}
	for name, rec := range cases {
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control = %q, want no-store", name, got)
		}
	}
}

func TestAdjustmentEnvelope(t *testing.T) {
	f := newWalletHTTPFixture(t)
	e := f.echo(t)
	path := "/api/v1/students/" + f.studentA.ID.String() + "/wallet/adjustments"
	key := "env-" + uuid.NewString()[:8]

	created := sendTryoutJSON(e, http.MethodPost, f.hostA, path, adjustmentBody("credit", "50000.00", "Hadiah juara kelas", key), f.tokenAdminA)
	if created.Code != http.StatusCreated {
		t.Fatalf("credit: status = %d body = %s, want 201", created.Code, created.Body.String())
	}
	first := decodeAdjustment(t, created)
	if first.Entry.Amount != "50000.00" || first.Entry.BalanceAfter != "50000.00" || first.Balance != "50000.00" {
		t.Errorf("adjustment = %+v, want 50000.00 on both the entry and the balance", first)
	}
	if first.Entry.Kind != "adjustment" || first.Entry.ActorKind != "staff" || first.Entry.ActorName == nil ||
		*first.Entry.ActorName != f.adminA.Name {
		t.Errorf("entry = %+v, want an adjustment by the admin with their name", first.Entry)
	}
	if first.Entry.SourceType != "adjustment" || first.Entry.SourceID != key {
		t.Errorf("entry source = %s/%s, want adjustment/%s", first.Entry.SourceType, first.Entry.SourceID, key)
	}

	replay := sendTryoutJSON(e, http.MethodPost, f.hostA, path, adjustmentBody("credit", "50000.00", "Hadiah juara kelas", key), f.tokenAdminA)
	if replay.Code != http.StatusOK {
		t.Fatalf("replay: status = %d body = %s, want 200", replay.Code, replay.Body.String())
	}
	again := decodeAdjustment(t, replay)
	if again.Entry.ID != first.Entry.ID || again.Balance != "50000.00" {
		t.Errorf("replay = %s / %s, want the original entry %s and the untouched balance", again.Entry.ID, again.Balance, first.Entry.ID)
	}

	conflict := sendTryoutJSON(e, http.MethodPost, f.hostA, path, adjustmentBody("credit", "60000.00", "Hadiah juara kelas", key), f.tokenAdminA)
	wantError(t, "the same key with another amount", conflict, http.StatusConflict, httpx.CodeConflict)

	for name, body := range map[string]map[string]any{
		"an empty reason": adjustmentBody("credit", "1000.00", "", "reason-"+uuid.NewString()[:8]),
		"a blank reason":  adjustmentBody("credit", "1000.00", "   ", "reason-"+uuid.NewString()[:8]),
		"an exponent":     adjustmentBody("credit", "1e3", "Hadiah", "amount-"+uuid.NewString()[:8]),
		"three decimals":  adjustmentBody("credit", "1.234", "Hadiah", "amount-"+uuid.NewString()[:8]),
		"zero":            adjustmentBody("credit", "0.00", "Hadiah", "amount-"+uuid.NewString()[:8]),
		"a negative":      adjustmentBody("credit", "-500.00", "Hadiah", "amount-"+uuid.NewString()[:8]),
		"an unknown type": adjustmentBody("grant", "1000.00", "Hadiah", "type-"+uuid.NewString()[:8]),
		"a short key":     adjustmentBody("credit", "1000.00", "Hadiah", "short"),
		"a dotted key":    adjustmentBody("credit", "1000.00", "Hadiah", "key."+uuid.NewString()[:8]),
		"a spaces key":    adjustmentBody("credit", "1000.00", "Hadiah", "key "+uuid.NewString()[:8]),
	} {
		rec := sendTryoutJSON(e, http.MethodPost, f.hostA, path, body, f.tokenAdminA)
		wantError(t, name, rec, http.StatusUnprocessableEntity, httpx.CodeValidation)
	}

	unknown := adjustmentBody("credit", "1000.00", "Hadiah", "unknown-"+uuid.NewString()[:8])
	unknown["note"] = "koreksi"
	wantError(t, "an unknown field", sendTryoutJSON(e, http.MethodPost, f.hostA, path, unknown, f.tokenAdminA),
		http.StatusUnprocessableEntity, httpx.CodeValidation, "body")

	over := sendTryoutJSON(e, http.MethodPost, f.hostA, path,
		adjustmentBody("debit", "50000.01", "Koreksi", "over-"+uuid.NewString()[:8]), f.tokenAdminA)
	wantError(t, "debit beyond the balance", over, http.StatusUnprocessableEntity, httpx.CodeValidation, "amount")
	if message := errorEnvelope(t, over).Error.Fields["amount"]; message != walletInsuff {
		t.Errorf("insufficient funds message = %q, want %q", message, walletInsuff)
	}
	if wallet := decodeWallet(t, getWithToken(e, f.hostA, "/api/v1/wallet", f.tokenA)); wallet.Balance != "50000.00" {
		t.Errorf("balance after the refused debit = %s, want 50000.00", wallet.Balance)
	}

	debit := sendTryoutJSON(e, http.MethodPost, f.hostA, path,
		adjustmentBody("debit", "50000", "Koreksi", "debit-"+uuid.NewString()[:8]), f.tokenAdminA)
	if debit.Code != http.StatusCreated {
		t.Fatalf("debit to zero: status = %d body = %s, want 201", debit.Code, debit.Body.String())
	}
	if body := decodeAdjustment(t, debit); body.Entry.Amount != "-50000.00" || body.Balance != "0.00" {
		t.Errorf("debit = %s / %s, want -50000.00 and 0.00", body.Entry.Amount, body.Balance)
	}

	for name, id := range map[string]string{
		"an unknown id":            uuid.NewString(),
		"an id that is not a uuid": "not-a-uuid",
	} {
		rec := sendTryoutJSON(e, http.MethodPost, f.hostA, "/api/v1/students/"+id+"/wallet/adjustments",
			adjustmentBody("credit", "1000.00", "Hadiah", "missing-"+uuid.NewString()[:8]), f.tokenAdminA)
		wantError(t, name, rec, http.StatusNotFound, httpx.CodeNotFound)
	}
}

func TestAdjustmentBodyIsBounded(t *testing.T) {
	f := newWalletHTTPFixture(t)
	e := f.echo(t)
	path := "/api/v1/students/" + f.studentA.ID.String() + "/wallet/adjustments"
	body := `{"type":"credit","amount":"1000.00","reason":"` + strings.Repeat("a", 64<<10) +
		`","idempotency_key":"` + uuid.NewString() + `"}`
	if len(body) < 8<<10 {
		t.Fatalf("the probe body is only %d bytes, so it cannot prove the cap", len(body))
	}
	rec := sendTryoutJSON(e, http.MethodPost, f.hostA, path, json.RawMessage(body), f.tokenAdminA)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversized body: status = %d body = %s, want 422", rec.Code, rec.Body.String())
	}
	if got := len(errorEnvelope(t, rec).Error.Fields); got > 2 {
		t.Fatalf("oversized body produced %d field errors, want it refused before it is mapped", got)
	}
	if wallet := decodeWallet(t, getWithToken(e, f.hostA, "/api/v1/wallet", f.tokenA)); wallet.Balance != "0.00" {
		t.Errorf("the oversized body moved money: balance = %s", wallet.Balance)
	}
}

// moneyString is the only shape money may take on the wire: a canonical decimal, never a number.
var moneyString = regexp.MustCompile(`^-?\d+\.\d{2}$`)

func TestWalletMoneyIsAlwaysAStringWithTwoDigits(t *testing.T) {
	f := newWalletHTTPFixture(t)
	e := f.echo(t)
	path := "/api/v1/students/" + f.studentA.ID.String() + "/wallet/adjustments"

	created := sendTryoutJSON(e, http.MethodPost, f.hostA, path,
		adjustmentBody("credit", "1234567.8", "Hadiah", "money-"+uuid.NewString()[:8]), f.tokenAdminA)
	if created.Code != http.StatusCreated {
		t.Fatalf("credit: status = %d body = %s", created.Code, created.Body.String())
	}
	key := "money-replay-" + uuid.NewString()[:8]
	first := sendTryoutJSON(e, http.MethodPost, f.hostA, path, adjustmentBody("debit", "0.01", "Koreksi", key), f.tokenAdminA)
	replay := sendTryoutJSON(e, http.MethodPost, f.hostA, path, adjustmentBody("debit", "0.01", "Koreksi", key), f.tokenAdminA)

	cases := map[string]*httptest.ResponseRecorder{
		"wallet":            getWithToken(e, f.hostA, "/api/v1/wallet", f.tokenA),
		"entries":           getWithToken(e, f.hostA, "/api/v1/wallet/entries", f.tokenA),
		"students":          getWithToken(e, f.hostA, "/api/v1/students", f.tokenAdminA),
		"student wallet":    getWithToken(e, f.hostA, "/api/v1/students/"+f.studentA.ID.String()+"/wallet", f.tokenAdminA),
		"student entries":   getWithToken(e, f.hostA, "/api/v1/students/"+f.studentA.ID.String()+"/wallet/entries", f.tokenAdminA),
		"adjustment":        created,
		"debit":             first,
		"adjustment replay": replay,
	}
	moneyFields := map[string]bool{
		"balance": true, "total_credits": true, "total_debits": true,
		"amount": true, "balance_after": true,
	}
	for name, rec := range cases {
		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
			t.Fatalf("%s: status = %d body = %s", name, rec.Code, rec.Body.String())
		}
		var body any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: body %s is not JSON: %v", name, rec.Body.String(), err)
		}
		seen := 0
		walkMoney(t, name, body, moneyFields, &seen)
		if seen == 0 {
			t.Errorf("%s carries no money field, so this check proves nothing: %s", name, rec.Body.String())
		}
	}
}

func walkMoney(t *testing.T, name string, node any, moneyFields map[string]bool, seen *int) {
	t.Helper()
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if moneyFields[key] {
				*seen++
				text, ok := child.(string)
				if !ok {
					t.Errorf("%s: %s = %v (%T), want a decimal string", name, key, child, child)
					continue
				}
				if !moneyString.MatchString(text) {
					t.Errorf("%s: %s = %q, want a canonical two-digit decimal string", name, key, text)
				}
				continue
			}
			walkMoney(t, name, child, moneyFields, seen)
		}
	case []any:
		for _, child := range value {
			walkMoney(t, name, child, moneyFields, seen)
		}
	}
}
