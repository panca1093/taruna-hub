package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

func TestNormalizeIdentifiers(t *testing.T) {
	emails := map[string]string{
		"naila@example.com":         "naila@example.com",
		"  Naila@Example.COM ":      "naila@example.com",
		"not-an-email":              "",
		"Naila <naila@example.com>": "",
		"":                          "",
	}
	for in, want := range emails {
		got, ok := NormalizeEmail(in)
		if ok != (want != "") || got != want {
			t.Errorf("NormalizeEmail(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	phones := map[string]string{
		"081234567890":       "+6281234567890",
		"0812-3456-7890":     "+6281234567890",
		"0812 3456 7890":     "+6281234567890",
		"6281234567890":      "+6281234567890",
		"+62 812 3456 7890":  "+6281234567890",
		"(0812) 3456.7890":   "+6281234567890",
		"+14155550100":       "+14155550100",
		"0812":               "",
		"+0812345678":        "",
		"0812abc4567":        "",
		"+62812345678901234": "",
	}
	for in, want := range phones {
		got, ok := NormalizePhone(in)
		if ok != (want != "") || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if got, _ := NormalizeIdentifier(" Naila@Example.com"); got != "naila@example.com" {
		t.Errorf("an identifier with @ read as %q, want the email form", got)
	}
	if got, _ := NormalizeIdentifier("0812-3456-7890"); got != "+6281234567890" {
		t.Errorf("an identifier without @ read as %q, want the phone form", got)
	}
}

// noJobs stands in for River: the notification row is the contract under test, not its delivery.
type noJobs struct{}

func (noJobs) InsertNotificationTx(context.Context, pgx.Tx, NotificationJobArgs) error { return nil }

var accountSealKey = bytes.Repeat([]byte{7}, SealKeyBytes)

type accountHarness struct {
	*authHarness
	account *AccountService
	sealer  *Sealer

	recipients []string
}

func newAccountHarness(t *testing.T, opts ...AuthOption) *accountHarness {
	t.Helper()
	h := &accountHarness{authHarness: newAuthHarness(t, opts...)}
	logger := slog.New(slog.NewJSONHandler(h.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	notifications, err := NewNotificationService(repository.NewNotificationRepository(h.pool), noJobs{}, nil, accountSealKey, logger, WithClock(h.clock.Now))
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}
	if h.sealer, err = NewSealer(accountSealKey); err != nil {
		t.Fatalf("sealer: %v", err)
	}
	h.account = NewAccountService(AccountDeps{
		DB:            h.pool,
		Users:         repository.NewUserRepository(h.pool),
		Sessions:      repository.NewSessionRepository(h.pool),
		Throttles:     repository.NewThrottleRepository(testRedis(t)),
		Notifications: notifications,
		Auth:          h.auth,
		Links:         WebLinks{Scheme: "https", BaseDomain: "tarunaeduka.test", Port: "3000"},
		Secret:        []byte(authSecretText),
		Logger:        logger,
	}, WithAccountClock(h.clock.Now), WithAccountPasswordCost(bcrypt.MinCost))
	// Registered after the auth harness, so it runs first: platform rows have no tenant to sweep by.
	t.Cleanup(func() {
		h.mu.Lock()
		recipients := append([]string(nil), h.recipients...)
		h.mu.Unlock()
		if _, err := h.pool.Exec(context.Background(),
			`DELETE FROM notification_logs WHERE tenant_id IS NULL AND recipient = ANY($1)`, recipients); err != nil {
			t.Errorf("cleanup platform notifications: %v", err)
		}
	})
	return h
}

func (h *accountHarness) track(recipient string) {
	h.mu.Lock()
	h.recipients = append(h.recipients, recipient)
	h.mu.Unlock()
}

// sealed opens the newest notification of one template for one recipient, the way the worker would.
func (h *accountHarness) sealed(recipient, template string) (map[string]string, model.Notification) {
	h.t.Helper()
	var n model.Notification
	var params map[string]string
	err := h.pool.QueryRow(context.Background(),
		`SELECT id, params, sealed_params, expires_at FROM notification_logs
		  WHERE recipient = $1 AND template = $2 ORDER BY created_at DESC, id LIMIT 1`,
		recipient, template).Scan(&n.ID, &params, &n.SealedParams, &n.ExpiresAt)
	if err != nil {
		h.t.Fatalf("read %s notification for %s: %v", template, recipient, err)
	}
	n.Params = params
	plain, err := h.sealer.Open(n.SealedParams, n.ID[:])
	if err != nil {
		h.t.Fatalf("open sealed params: %v", err)
	}
	var secret map[string]string
	if err := json.Unmarshal(plain, &secret); err != nil {
		h.t.Fatalf("decode sealed params: %v", err)
	}
	return secret, n
}

func (h *accountHarness) code(email string) string {
	h.t.Helper()
	secret, _ := h.sealed(email, TemplateEmailCode)
	return secret["code"]
}

func (h *accountHarness) notificationCount(recipient string) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM notification_logs WHERE recipient = $1`, recipient).Scan(&n); err != nil {
		h.t.Fatalf("count notifications: %v", err)
	}
	return n
}

func (h *accountHarness) register(scope HostScope, email, phone string) Tokens {
	h.t.Helper()
	tokens, err := h.account.Register(context.Background(), scope, RegisterInput{
		Name: "Budi Santoso", Email: email, Phone: phone, Password: authPassword,
	})
	if err != nil {
		h.t.Fatalf("register %s: %v", email, err)
	}
	return tokens
}

func uniqueEmail(prefix string) string { return prefix + "-" + uuid.NewString()[:8] + "@example.com" }

// uniqueLocalPhone is an Indonesian local-format number no other test uses.
func uniqueLocalPhone() string {
	return fmt.Sprintf("0812%08d", time.Now().UnixNano()%100_000_000)
}

func otherCode(code string) string {
	if code == "000000" {
		return "000001"
	}
	return "000000"
}

// principal is what the Authenticate middleware would hand the handlers for these tokens.
func (h *accountHarness) principal(scope HostScope, tokens Tokens) model.Principal {
	h.t.Helper()
	p, err := h.authenticate(scope, tokens.AccessToken)
	if err != nil {
		h.t.Fatalf("authenticate: %v", err)
	}
	return p
}

// D01: signup creates the account and logs in at once; the code only flips a flag the web app shows.
func TestRegisterCreatesAStudentWhoCanLogInBeforeVerifying(t *testing.T) {
	h := newAccountHarness(t)
	tenant := h.tenant()
	ctx := context.Background()
	email := uniqueEmail("signup")
	phone := uniqueLocalPhone()
	canonicalPhone := "+62" + phone[1:]

	tokens, err := h.account.Register(ctx, h.scope(tenant), RegisterInput{
		Name: "  Budi Santoso ", Email: "  " + strings.ToUpper(email), Phone: phone[:4] + "-" + phone[4:], Password: authPassword,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	p := tokens.Principal
	if p.Role != model.RoleStudent || p.Name != "Budi Santoso" || p.Email == nil || *p.Email != email ||
		p.Phone == nil || *p.Phone != canonicalPhone || p.EmailVerified {
		t.Errorf("principal = %+v, want an unverified student with the canonical email and phone", p)
	}
	if p.TenantID == nil || *p.TenantID != tenant.ID || tokens.RefreshToken == "" {
		t.Errorf("principal tenant = %v, refresh = %q; want %s and a session", p.TenantID, tokens.RefreshToken, tenant.ID)
	}

	// Unverified is not locked out: every spelling of the identifiers logs in.
	for _, identifier := range []string{canonicalPhone, phone, "62" + phone[1:], strings.ToUpper(email)} {
		got, err := h.auth.Login(ctx, h.scope(tenant), identifier, authPassword)
		if err != nil {
			t.Errorf("login as %q before verifying: %v", identifier, err)
			continue
		}
		if got.Principal.UserID != p.UserID || got.Principal.EmailVerified {
			t.Errorf("login as %q = %+v, want the same unverified account", identifier, got.Principal)
		}
	}

	secret, n := h.sealed(email, TemplateEmailCode)
	code := secret["code"]
	if !validCodeShape(code) {
		t.Fatalf("code %q is not %d digits", code, codeDigits)
	}
	if _, leaked := n.Params["code"]; leaked {
		t.Error("the code is stored in clear params")
	}
	if n.ExpiresAt == nil || !n.ExpiresAt.Equal(h.clock.Now().Add(codeTTL)) {
		t.Errorf("notification expires_at = %v, want the code expiry", n.ExpiresAt)
	}

	if err := h.account.VerifyEmail(ctx, p, code); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !h.principal(h.scope(tenant), tokens).EmailVerified {
		t.Error("the live session does not see the verification")
	}
	if err := h.account.VerifyEmail(ctx, p, otherCode(code)); err != nil {
		t.Errorf("verifying an already verified email: err = %v, want nil", err)
	}
	if err := h.account.ResendEmailCode(ctx, h.scope(tenant), p); !errors.Is(err, ErrAlreadyVerified) {
		t.Errorf("resend after verifying: err = %v, want ErrAlreadyVerified", err)
	}
	var hash []byte
	if err := h.pool.QueryRow(ctx, `SELECT email_code_hash FROM users WHERE id = $1`, p.UserID).Scan(&hash); err != nil || hash != nil {
		t.Errorf("code hash after verifying = %x, %v; want it cleared", hash, err)
	}
	if strings.Contains(h.logs.String(), code) {
		t.Error("the code was written to a log")
	}
}

// FR-AUTH-07: the same email and phone register separately in two tenants.
func TestRegisterSameIdentifiersInTwoTenants(t *testing.T) {
	h := newAccountHarness(t)
	a, b := h.tenant(), h.tenant()
	email, phone := uniqueEmail("twin"), uniqueLocalPhone()
	ta := h.register(h.scope(a), email, phone)
	tb := h.register(h.scope(b), email, phone)
	if ta.Principal.UserID == tb.Principal.UserID {
		t.Error("two tenants share one account")
	}
	// A's code is bound to A's account, so it verifies nothing on B.
	codeA := h.codeFor(a.ID, email)
	if err := h.account.VerifyEmail(context.Background(), tb.Principal, codeA); err == nil && h.codeFor(b.ID, email) != codeA {
		t.Error("A's code verified B's account")
	}
}

func (h *accountHarness) codeFor(tenantID uuid.UUID, recipient string) string {
	h.t.Helper()
	var n model.Notification
	if err := h.pool.QueryRow(context.Background(),
		`SELECT id, sealed_params FROM notification_logs WHERE tenant_id = $1 AND recipient = $2 AND template = $3
		  ORDER BY created_at DESC, id LIMIT 1`, tenantID, recipient, TemplateEmailCode).Scan(&n.ID, &n.SealedParams); err != nil {
		h.t.Fatalf("read code notification: %v", err)
	}
	plain, err := h.sealer.Open(n.SealedParams, n.ID[:])
	if err != nil {
		h.t.Fatalf("open: %v", err)
	}
	var secret map[string]string
	if err := json.Unmarshal(plain, &secret); err != nil {
		h.t.Fatalf("decode: %v", err)
	}
	return secret["code"]
}

// An account exists from the moment of signup, verified or not, so its identifiers are taken.
func TestRegisterRefusesTakenIdentifiersAndBadInput(t *testing.T) {
	h := newAccountHarness(t)
	tenant := h.tenant()
	ctx := context.Background()
	existing := h.user(&tenant.ID, model.RoleStudent, authPassword)
	unverifiedEmail, phone := uniqueEmail("unverified"), uniqueLocalPhone()
	h.register(h.scope(tenant), unverifiedEmail, phone)

	cases := []struct {
		name string
		in   RegisterInput
		want error
	}{
		{"email in another case", RegisterInput{Name: "X", Email: strings.ToUpper(*existing.Email), Password: authPassword}, ErrEmailTaken},
		{"an unverified account's email", RegisterInput{Name: "X", Email: unverifiedEmail, Password: "other-password"}, ErrEmailTaken},
		{"phone in another form", RegisterInput{Name: "X", Email: uniqueEmail("fresh"), Phone: "62" + phone[1:], Password: authPassword}, ErrPhoneTaken},
	}
	for _, tc := range cases {
		if _, err := h.account.Register(ctx, h.scope(tenant), tc.in); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	// The squatter's signup did not change who owns the unverified account's password.
	h.login(h.scope(tenant), unverifiedEmail, authPassword)

	_, err := h.account.Register(ctx, h.scope(tenant), RegisterInput{Name: " ", Email: "not-an-email", Phone: "12", Password: "short"})
	var input *InputError
	if !errors.As(err, &input) {
		t.Fatalf("bad input: err = %v, want an InputError", err)
	}
	want := map[string]string{"name": ReasonInvalid, "email": ReasonInvalid, "phone": ReasonInvalid, "password": ReasonPassword}
	for field, reason := range want {
		if input.Fields[field] != reason {
			t.Errorf("field %s = %q, want %q", field, input.Fields[field], reason)
		}
	}
	if _, err := h.account.Register(ctx, h.scope(tenant), RegisterInput{
		Name: "X", Email: uniqueEmail("long"), Password: strings.Repeat("a", maxPasswordBytes+1),
	}); !errors.As(err, &input) {
		t.Errorf("a 73-byte password: err = %v, want an InputError", err)
	}
}

// Two signups for one email race; the unique constraint lets exactly one through.
func TestConcurrentRegisterCreatesOneAccount(t *testing.T) {
	h := newAccountHarness(t)
	tenant := h.tenant()
	email := uniqueEmail("race")
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.account.Register(context.Background(), h.scope(tenant), RegisterInput{Name: "R", Email: email, Password: authPassword})
			switch {
			case err == nil:
				wins.Add(1)
			case !errors.Is(err, ErrEmailTaken):
				t.Errorf("a losing signup: err = %v, want ErrEmailTaken", err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Errorf("successful signups = %d, want 1", wins.Load())
	}
}

// Platform Admins are provisioned; the platform host offers no signup at all.
func TestRegistrationIsClosedOnThePlatformHost(t *testing.T) {
	h := newAccountHarness(t)
	email := uniqueEmail("platform-signup")
	if _, err := h.account.Register(context.Background(), h.platformScope(), RegisterInput{Name: "X", Email: email, Password: authPassword}); !errors.Is(err, ErrRegistrationClosed) {
		t.Errorf("register: err = %v, want ErrRegistrationClosed", err)
	}
	if n := h.notificationCount(email); n != 0 {
		t.Errorf("notifications = %d, want 0", n)
	}
}

// Guesses are counted under the row lock, so a burst cannot buy more than maxCodeAttempts tries.
func TestWrongCodesAreCountedUnderConcurrency(t *testing.T) {
	h := newAccountHarness(t)
	tenant := h.tenant()
	ctx := context.Background()
	email := uniqueEmail("guess")
	p := h.register(h.scope(tenant), email, "").Principal
	code := h.code(email)

	var wg sync.WaitGroup
	for range 3 * maxCodeAttempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.account.VerifyEmail(ctx, p, otherCode(code)); !errors.Is(err, ErrInvalidCode) {
				t.Errorf("wrong code: err = %v, want ErrInvalidCode", err)
			}
		}()
	}
	wg.Wait()

	var attempts int
	if err := h.pool.QueryRow(ctx, `SELECT email_code_attempts FROM users WHERE id = $1`, p.UserID).Scan(&attempts); err != nil {
		t.Fatalf("read attempts: %v", err)
	}
	if attempts != maxCodeAttempts {
		t.Errorf("attempts = %d, want exactly %d", attempts, maxCodeAttempts)
	}
	if err := h.account.VerifyEmail(ctx, p, code); !errors.Is(err, ErrInvalidCode) {
		t.Errorf("the right code after the budget is spent: err = %v, want ErrInvalidCode", err)
	}

	// A fresh code restores the budget, and only the fresh code works.
	h.clock.Advance(codeResendAfter + time.Second)
	if err := h.account.ResendEmailCode(ctx, h.scope(tenant), p); err != nil {
		t.Fatalf("resend: %v", err)
	}
	fresh := h.code(email)
	if fresh != code {
		if err := h.account.VerifyEmail(ctx, p, code); !errors.Is(err, ErrInvalidCode) {
			t.Errorf("the replaced code: err = %v, want ErrInvalidCode", err)
		}
	}
	if err := h.account.VerifyEmail(ctx, p, fresh); err != nil {
		t.Errorf("the fresh code: %v", err)
	}
}

func TestCodeExpiresAndSendsArePacedAndCapped(t *testing.T) {
	h := newAccountHarness(t)
	tenant := h.tenant()
	ctx := context.Background()
	email := uniqueEmail("pace")
	p := h.register(h.scope(tenant), email, "").Principal
	code := h.code(email)

	var limited *ThrottleError
	if err := h.account.ResendEmailCode(ctx, h.scope(tenant), p); !errors.As(err, &limited) || limited.RetryAfter <= 0 || limited.RetryAfter > codeResendAfter {
		t.Errorf("resend inside the cooldown: err = %v, want a ThrottleError within %s", err, codeResendAfter)
	}
	h.clock.Advance(codeTTL)
	if err := h.account.VerifyEmail(ctx, p, code); !errors.Is(err, ErrInvalidCode) {
		t.Errorf("a code at its expiry: err = %v, want ErrInvalidCode", err)
	}

	// The signup sent one and the refused resend still counted, so three more fit in the hour.
	for i := range codeThrottle.limit - 2 {
		h.clock.Advance(codeResendAfter)
		if err := h.account.ResendEmailCode(ctx, h.scope(tenant), p); err != nil {
			t.Fatalf("resend %d: %v", i+1, err)
		}
	}
	h.clock.Advance(codeResendAfter)
	if err := h.account.ResendEmailCode(ctx, h.scope(tenant), p); !errors.As(err, &limited) || limited.RetryAfter > codeThrottle.window {
		t.Errorf("a send past the hourly cap: err = %v, want a ThrottleError", err)
	}
}

func (h *accountHarness) resetToken(scope HostScope, email string) string {
	h.t.Helper()
	secret, n := h.sealed(email, TemplatePasswordReset)
	label := "admin"
	if !scope.Platform {
		label = scope.Tenant.Slug
	}
	prefix := "https://" + label + ".tarunaeduka.test:3000/reset-password#token="
	link := secret["reset_url"]
	if !strings.HasPrefix(link, prefix) {
		h.t.Fatalf("reset link %q, want it under %q", link, prefix)
	}
	if _, leaked := n.Params["reset_url"]; leaked {
		h.t.Error("the reset link is stored in clear params")
	}
	return strings.TrimPrefix(link, prefix)
}

// A reset proves the inbox, so it also verifies the email: the real owner's way back from a squatter.
func TestPasswordResetSetsPasswordEndsSessionsAndVerifiesEmail(t *testing.T) {
	h := newAccountHarness(t)
	tenant, other := h.tenant(), h.tenant()
	ctx := context.Background()
	email := uniqueEmail("reset")
	first := h.register(h.scope(tenant), email, "")
	second := h.login(h.scope(tenant), email, authPassword)

	if err := h.account.ForgotPassword(ctx, h.scope(tenant), strings.ToUpper(email)); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	token := h.resetToken(h.scope(tenant), email)

	const newPassword = "brand-new-horse"
	if err := h.account.ResetPassword(ctx, h.scope(other), token, newPassword); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("the token on another tenant's host: err = %v, want ErrInvalidResetToken", err)
	}
	var input *InputError
	if err := h.account.ResetPassword(ctx, h.scope(tenant), token, "short"); !errors.As(err, &input) {
		t.Errorf("a short password: err = %v, want an InputError", err)
	}
	if err := h.account.ResetPassword(ctx, h.scope(tenant), token, newPassword); err != nil {
		t.Fatalf("reset: %v", err)
	}

	for name, tokens := range map[string]Tokens{"first": first, "second": second} {
		if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("%s session after the reset: err = %v, want ErrUnauthenticated", name, err)
		}
	}
	if _, err := h.auth.Login(ctx, h.scope(tenant), email, authPassword); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("the old password: err = %v, want ErrInvalidCredentials", err)
	}
	if !h.login(h.scope(tenant), email, newPassword).Principal.EmailVerified {
		t.Error("a completed reset did not verify the email")
	}
	if err := h.account.ResetPassword(ctx, h.scope(tenant), token, "yet-another-horse"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("the token a second time: err = %v, want ErrInvalidResetToken", err)
	}
	if strings.Contains(h.logs.String(), token) {
		t.Error("the reset token was written to a log")
	}
}

// The token is bound to the password hash: whichever link is used first kills every other one.
func TestResetTokensAreSingleUseUnderConcurrencyAndExpire(t *testing.T) {
	h := newAccountHarness(t)
	tenant := h.tenant()
	ctx := context.Background()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)

	if err := h.account.ForgotPassword(ctx, h.scope(tenant), *user.Email); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	older := h.resetToken(h.scope(tenant), *user.Email)
	h.clock.Advance(time.Second) // a different expiry, so the two links differ
	if err := h.account.ForgotPassword(ctx, h.scope(tenant), *user.Email); err != nil {
		t.Fatalf("forgot again: %v", err)
	}
	token := h.resetToken(h.scope(tenant), *user.Email)
	if token == older {
		t.Fatal("two requests produced the same link")
	}

	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := h.account.ResetPassword(ctx, h.scope(tenant), token, fmt.Sprintf("racing-horse-%d", i))
			switch {
			case err == nil:
				wins.Add(1)
			case !errors.Is(err, ErrInvalidResetToken):
				t.Errorf("a losing reset: err = %v, want ErrInvalidResetToken", err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Errorf("successful resets = %d, want 1", wins.Load())
	}
	if err := h.account.ResetPassword(ctx, h.scope(tenant), older, "brand-new-horse"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("the other link after one was used: err = %v, want ErrInvalidResetToken", err)
	}

	h.clock.Advance(16 * time.Minute) // past the forgot window, so another request is allowed
	if err := h.account.ForgotPassword(ctx, h.scope(tenant), *user.Email); err != nil {
		t.Fatalf("forgot a third time: %v", err)
	}
	expiring := h.resetToken(h.scope(tenant), *user.Email)
	h.clock.Advance(resetTTL)
	if err := h.account.ResetPassword(ctx, h.scope(tenant), expiring, "late-horse-123"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("a token at its expiry: err = %v, want ErrInvalidResetToken", err)
	}
}

// A token is a claim the server re-derives; any edit to it, or a made-up one, is refused.
func TestResetTokensCannotBeForged(t *testing.T) {
	h := newAccountHarness(t)
	tenant := h.tenant()
	ctx := context.Background()
	victim := h.user(&tenant.ID, model.RoleStudent, authPassword)
	attacker := h.user(&tenant.ID, model.RoleStudent, authPassword)

	if err := h.account.ForgotPassword(ctx, h.scope(tenant), *attacker.Email); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	token := h.resetToken(h.scope(tenant), *attacker.Email)
	_, rawMAC, _ := strings.Cut(token, ".")
	// The attacker's valid MAC on a payload naming the victim.
	forged := base64.RawURLEncoding.EncodeToString(resetPayload(victim.ID, h.clock.Now().Add(resetTTL))) + "." + rawMAC
	flipped := []byte(token)
	flipped[len(flipped)-2] ^= 1

	for name, bad := range map[string]string{
		"another user's id": forged, "a flipped byte": string(flipped), "no separator": strings.ReplaceAll(token, ".", ""),
		"garbage": "not-a-token", "empty mac": strings.SplitN(token, ".", 2)[0] + ".",
	} {
		if err := h.account.ResetPassword(ctx, h.scope(tenant), bad, "hijacked-horse"); !errors.Is(err, ErrInvalidResetToken) {
			t.Errorf("%s: err = %v, want ErrInvalidResetToken", name, err)
		}
	}
	h.login(h.scope(tenant), *victim.Email, authPassword)
}

// Forgot answers the same for everyone, and its throttle is keyed on the input, not on an account.
func TestForgotPasswordRevealsNothing(t *testing.T) {
	h := newAccountHarness(t)
	tenant := h.tenant()
	ctx := context.Background()

	unknown := uniqueEmail("ghost")
	if err := h.account.ForgotPassword(ctx, h.scope(tenant), unknown); err != nil {
		t.Errorf("unknown email: err = %v, want nil", err)
	}
	inactive := h.user(&tenant.ID, model.RoleStudent, authPassword)
	h.setActive(inactive.ID, false)
	if err := h.account.ForgotPassword(ctx, h.scope(tenant), *inactive.Email); err != nil {
		t.Errorf("inactive account: err = %v, want nil", err)
	}
	for _, email := range []string{unknown, *inactive.Email} {
		if n := h.notificationCount(email); n != 0 {
			t.Errorf("notifications for %s = %d, want 0", email, n)
		}
	}

	for i := 1; i < forgotThrottle.limit; i++ {
		if err := h.account.ForgotPassword(ctx, h.scope(tenant), unknown); err != nil {
			t.Fatalf("forgot %d: %v", i+1, err)
		}
	}
	var limited *ThrottleError
	if err := h.account.ForgotPassword(ctx, h.scope(tenant), unknown); !errors.As(err, &limited) {
		t.Errorf("forgot past the limit for an unknown email: err = %v, want a ThrottleError", err)
	}
}

func TestPlatformPasswordResetStaysOnThePlatformHost(t *testing.T) {
	h := newAccountHarness(t)
	tenant := h.tenant()
	ctx := context.Background()
	admin := h.user(nil, model.RolePlatformAdmin, authPassword)
	h.track(*admin.Email)

	if err := h.account.ForgotPassword(ctx, h.platformScope(), *admin.Email); err != nil {
		t.Fatalf("forgot: %v", err)
	}
	token := h.resetToken(h.platformScope(), *admin.Email)
	if err := h.account.ResetPassword(ctx, h.scope(tenant), token, "platform-new-horse"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("a platform token on a tenant host: err = %v, want ErrInvalidResetToken", err)
	}
	if err := h.account.ResetPassword(ctx, h.platformScope(), token, "platform-new-horse"); err != nil {
		t.Fatalf("reset on the platform host: %v", err)
	}
	h.login(h.platformScope(), *admin.Email, "platform-new-horse")
}

func TestLoginThrottleCountsEverySpellingAndForgivesSuccess(t *testing.T) {
	h := newAccountHarness(t)
	tenant, other := h.tenant(), h.tenant()
	ctx := context.Background()
	phone := uniqueLocalPhone()
	canonical := "+62" + phone[1:]
	email := uniqueEmail("throttled")
	user, err := repository.NewUserRepository(h.pool).Create(ctx, model.User{
		TenantID: &tenant.ID, Name: "T", Email: &email, Phone: &canonical, PasswordHash: h.hashed(authPassword), Role: model.RoleStudent,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	twin := h.userWithEmail(&other.ID, model.RoleStudent, email, h.hashed(authPassword))

	// Every spelling normalizes to one key, so alternating them buys nothing.
	spellings := []string{phone, canonical, "62" + phone[1:]}
	for i := range loginThrottle.limit {
		if _, err := h.auth.Login(ctx, h.scope(tenant), spellings[i%len(spellings)], "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("wrong attempt %d: err = %v, want ErrInvalidCredentials", i+1, err)
		}
	}
	var limited *ThrottleError
	if _, err := h.auth.Login(ctx, h.scope(tenant), canonical, authPassword); !errors.As(err, &limited) {
		t.Fatalf("the right password past the limit: err = %v, want a ThrottleError", err)
	}
	if limited.RetryAfter <= 0 || limited.RetryAfter > loginThrottle.window {
		t.Errorf("retry after %s, want within %s", limited.RetryAfter, loginThrottle.window)
	}
	// The email is a separate key, and another tenant's account is untouched.
	if got := h.login(h.scope(tenant), email, authPassword); got.Principal.UserID != user.ID {
		t.Errorf("email login reached %s, want %s", got.Principal.UserID, user.ID)
	}
	if got := h.login(h.scope(other), email, authPassword); got.Principal.UserID != twin.ID {
		t.Errorf("the other tenant reached %s, want %s", got.Principal.UserID, twin.ID)
	}

	h.clock.Advance(loginThrottle.window)
	h.login(h.scope(tenant), canonical, authPassword)

	// That success cleared the counter, so a fresh run of typos gets the whole budget again.
	for i := range loginThrottle.limit {
		if _, err := h.auth.Login(ctx, h.scope(tenant), canonical, "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("typo %d after a success: err = %v, want ErrInvalidCredentials", i+1, err)
		}
	}
}

// Counting happens before the compare, so a burst gets exactly the budget and no more.
func TestLoginThrottleHoldsUnderConcurrency(t *testing.T) {
	var compares atomic.Int32
	h := newAccountHarness(t, WithAuthPasswordCompare(func(hash, password []byte) error {
		compares.Add(1)
		return bcrypt.CompareHashAndPassword(hash, password)
	}))
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	compares.Store(0)

	var throttled atomic.Int32
	var wg sync.WaitGroup
	for range 3 * loginThrottle.limit {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.auth.Login(context.Background(), h.scope(tenant), *user.Email, "wrong-password")
			var limited *ThrottleError
			if errors.As(err, &limited) {
				throttled.Add(1)
			} else if !errors.Is(err, ErrInvalidCredentials) {
				t.Errorf("err = %v", err)
			}
		}()
	}
	wg.Wait()
	if got := compares.Load(); got != int32(loginThrottle.limit) {
		t.Errorf("password compares = %d, want exactly %d", got, loginThrottle.limit)
	}
	if got := throttled.Load(); got != int32(2*loginThrottle.limit) {
		t.Errorf("throttled = %d, want %d", got, 2*loginThrottle.limit)
	}
}

func TestRefreshReplayInsideGraceKeepsTheCookie(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	ctx := context.Background()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	first := h.login(h.scope(tenant), *user.Email, authPassword)

	second, err := h.auth.Refresh(ctx, h.scope(tenant), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	h.clock.Advance(refreshReuseGrace - time.Second)
	again, err := h.auth.Refresh(ctx, h.scope(tenant), first.RefreshToken)
	if err != nil {
		t.Fatalf("the replaced token inside the grace window: %v", err)
	}
	if again.RefreshToken != "" {
		t.Error("a grace refresh issued another refresh token; the cookie from the winning refresh must stand")
	}
	if _, err := h.authenticate(h.scope(tenant), again.AccessToken); err != nil {
		t.Errorf("the grace refresh's access token was rejected: %v", err)
	}
	if !bytes.Equal(h.sessionRow(second.RefreshToken).RefreshHash, hashRefreshToken(second.RefreshToken)) {
		t.Error("a grace refresh moved the current token")
	}
}

// Replayed after the grace window, a replaced token can only be a copy: the whole session goes.
func TestRefreshReplayAfterGraceRevokesTheSession(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	ctx := context.Background()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	first := h.login(h.scope(tenant), *user.Email, authPassword)

	second, err := h.auth.Refresh(ctx, h.scope(tenant), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	h.clock.Advance(refreshReuseGrace + time.Second)
	if _, err := h.auth.Refresh(ctx, h.scope(tenant), first.RefreshToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("a replay after the grace window: err = %v, want ErrUnauthenticated", err)
	}
	if _, err := h.auth.Refresh(ctx, h.scope(tenant), second.RefreshToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("the legitimate current token after a replay: err = %v, want ErrUnauthenticated", err)
	}
	if _, err := h.authenticate(h.scope(tenant), second.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("the access token after a replay: err = %v, want ErrUnauthenticated", err)
	}
	if !strings.Contains(h.logs.String(), "refresh token reused") {
		t.Error("the replay left no log line for the operator")
	}
}

// Two tabs refresh at once: both succeed, exactly one rotates, and the stored token is that one.
func TestConcurrentRefreshRotatesOnce(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	first := h.login(h.scope(tenant), *user.Email, authPassword)

	var mu sync.Mutex
	var issued []string
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tokens, err := h.auth.Refresh(context.Background(), h.scope(tenant), first.RefreshToken)
			if err != nil {
				t.Errorf("a racing refresh: %v", err)
				return
			}
			if tokens.RefreshToken != "" {
				mu.Lock()
				issued = append(issued, tokens.RefreshToken)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(issued) != 1 {
		t.Fatalf("rotations = %d, want 1", len(issued))
	}
	if !bytes.Equal(h.sessionRow(issued[0]).RefreshHash, hashRefreshToken(issued[0])) {
		t.Error("the stored token is not the one the winning refresh issued")
	}
}

func TestLogoutWithTheReplacedTokenEndsTheSession(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	ctx := context.Background()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	first := h.login(h.scope(tenant), *user.Email, authPassword)
	second, err := h.auth.Refresh(ctx, h.scope(tenant), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if err := h.auth.Logout(ctx, h.scope(tenant), first.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := h.auth.Refresh(ctx, h.scope(tenant), second.RefreshToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("refresh after logout with the replaced token: err = %v, want ErrUnauthenticated", err)
	}
}
