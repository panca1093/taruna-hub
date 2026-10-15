package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

// authSecretText is long enough for HMAC-SHA256; a deployment gets this from JWT_SECRET.
const authSecretText = "p03a-service-test-secret-0123456789"

const (
	authPassword    = "correct-horse"
	authOtherPasswd = "another-horse"
)

// testClock is the injected now(); every session rule is time-based, so tests move it instead of waiting.
type testClock struct {
	mu sync.Mutex
	at time.Time
}

func newTestClock() *testClock {
	return &testClock{at: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

type authHarness struct {
	t       *testing.T
	pool    *pgxpool.Pool
	tenants *TenantService
	auth    *AuthService
	clock   *testClock
	logs    *syncBuffer

	mu       sync.Mutex
	ownedTen []uuid.UUID
	ownedUsr []uuid.UUID
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newAuthHarness(t *testing.T, opts ...AuthOption) *authHarness {
	t.Helper()
	tenants, pool := testTenantService(t)
	logs := &syncBuffer{}
	h := &authHarness{
		t:       t,
		pool:    pool,
		tenants: tenants,
		clock:   newTestClock(),
		logs:    logs,
	}
	// The injected clock goes first so a test may override it through opts.
	all := append([]AuthOption{WithAuthClock(h.clock.Now)}, opts...)
	h.auth = NewAuthService(
		repository.NewUserRepository(pool),
		repository.NewSessionRepository(pool),
		repository.NewThrottleRepository(pool),
		[]byte(authSecretText),
		slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
		all...,
	)
	t.Cleanup(h.cleanup)
	return h
}

func (h *authHarness) cleanup() {
	ctx := context.Background()
	h.mu.Lock()
	tenants, users := append([]uuid.UUID(nil), h.ownedTen...), append([]uuid.UUID(nil), h.ownedUsr...)
	h.mu.Unlock()

	// Only the rows this harness created, sessions before the users they reference.
	if _, err := h.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE tenant_id = ANY($1) OR user_id = ANY($2)`, tenants, users); err != nil {
		h.t.Errorf("cleanup sessions: %v", err)
	}
	if len(users) > 0 {
		if _, err := h.pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, users); err != nil {
			h.t.Errorf("cleanup users: %v", err)
		}
	}
	if len(tenants) > 0 {
		if _, err := h.pool.Exec(ctx, `DELETE FROM users WHERE tenant_id = ANY($1)`, tenants); err != nil {
			h.t.Errorf("cleanup tenant users: %v", err)
		}
		if _, err := h.pool.Exec(ctx, `DELETE FROM tenants WHERE id = ANY($1)`, tenants); err != nil {
			h.t.Errorf("cleanup tenants: %v", err)
		}
	}
}

// tenant creates a second active tenant this harness owns.
func (h *authHarness) tenant() model.Tenant {
	h.t.Helper()
	tenant := provisionTenant(h.t, h.tenants, h.pool)
	h.mu.Lock()
	h.ownedTen = append(h.ownedTen, tenant.ID)
	h.mu.Unlock()
	return tenant
}

// user creates an account with a known password; nil tenantID is the platform scope.
func (h *authHarness) user(tenantID *uuid.UUID, role model.Role, password string) model.User {
	h.t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		h.t.Fatalf("hash password: %v", err)
	}
	email := "auth-" + uuid.NewString()[:8] + "@example.com"
	return h.userWithEmail(tenantID, role, email, string(hash))
}

func (h *authHarness) userWithEmail(tenantID *uuid.UUID, role model.Role, email, hash string) model.User {
	h.t.Helper()
	user, err := repository.NewUserRepository(h.pool).Create(context.Background(), model.User{
		TenantID: tenantID, Name: "Auth Probe", Email: &email, PasswordHash: hash, Role: role,
	})
	if err != nil {
		h.t.Fatalf("create user: %v", err)
	}
	h.mu.Lock()
	h.ownedUsr = append(h.ownedUsr, user.ID)
	h.mu.Unlock()
	return user
}

// hashed is a bcrypt hash of the given password, for a user a test wants to log in as.
func (h *authHarness) hashed(password string) string {
	h.t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		h.t.Fatalf("hash password: %v", err)
	}
	return string(hash)
}

func (h *authHarness) scope(tenant model.Tenant) HostScope { return HostScope{Tenant: tenant} }
func (h *authHarness) platformScope() HostScope            { return HostScope{Platform: true} }

// login is the one way a test gets a real token pair.
func (h *authHarness) login(scope HostScope, identifier, password string) Tokens {
	h.t.Helper()
	tokens, err := h.auth.Login(context.Background(), scope, identifier, password)
	if err != nil {
		h.t.Fatalf("login %s: %v", identifier, err)
	}
	return tokens
}

func (h *authHarness) authenticate(scope HostScope, token string) (model.Principal, error) {
	return h.auth.Authenticate(context.Background(), scope, token)
}

// sessionRow reads the stored session behind a refresh token, so a test can inspect the real row.
func (h *authHarness) sessionRow(refreshToken string) model.Session {
	h.t.Helper()
	session, err := repository.NewSessionRepository(h.pool).FindByRefreshHash(context.Background(), hashRefreshToken(refreshToken))
	if err != nil {
		h.t.Fatalf("read session: %v", err)
	}
	return session
}

// signed mints a token for real claims, so a test can break exactly one thing and nothing else.
func (h *authHarness) signed(method jwt.SigningMethod, p model.Principal, mutate func(*accessClaims)) string {
	h.t.Helper()
	now := h.clock.Now()
	claims := accessClaims{
		SessionID: p.SessionID,
		TenantID:  tenantLabel(p.TenantID),
		Role:      p.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    accessTokenIssuer,
			Subject:   p.UserID.String(),
			Audience:  jwt.ClaimStrings{accessTokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	}
	if mutate != nil {
		mutate(&claims)
	}
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(authSecretText))
	if err != nil {
		h.t.Fatalf("sign token: %v", err)
	}
	return token
}

// liveAccessToken proves a rejection was not merely an expired JWT.
func (h *authHarness) liveAccessToken(token string) bool {
	var claims accessClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil {
		h.t.Fatalf("decode access token: %v", err)
	}
	if claims.ExpiresAt == nil {
		h.t.Fatal("access token carries no exp")
	}
	return claims.ExpiresAt.After(h.clock.Now())
}

func (h *authHarness) setRole(userID uuid.UUID, role model.Role) {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), `UPDATE users SET role = $2 WHERE id = $1`, userID, role); err != nil {
		h.t.Fatalf("set role: %v", err)
	}
}

func (h *authHarness) setActive(userID uuid.UUID, active bool) {
	h.t.Helper()
	if _, err := h.pool.Exec(context.Background(), `UPDATE users SET is_active = $2 WHERE id = $1`, userID, active); err != nil {
		h.t.Fatalf("set is_active: %v", err)
	}
}

// One identifier may exist in several tenants: the host decides which account is meant.
func TestLoginSameIdentifierTwoTenants(t *testing.T) {
	h := newAuthHarness(t)
	shared := "shared-" + uuid.NewString()[:8] + "@example.com"
	a, b := h.tenant(), h.tenant()
	userA := h.userWithEmail(&a.ID, model.RoleStudent, shared, h.hashed(authPassword))
	userB := h.userWithEmail(&b.ID, model.RoleStudent, shared, h.hashed(authOtherPasswd))

	onlyA := "only-a-" + uuid.NewString()[:8] + "@example.com"
	onlyAUser := h.userWithEmail(&a.ID, model.RoleStudent, onlyA, h.hashed(authPassword))

	cases := []struct {
		name       string
		scope      HostScope
		identifier string
		password   string
		want       uuid.UUID
		wantErr    error
	}{
		{"A's own credentials on A", h.scope(a), shared, authPassword, userA.ID, nil},
		{"B's own credentials on B", h.scope(b), shared, authOtherPasswd, userB.ID, nil},
		{"A's password on B", h.scope(b), shared, authPassword, uuid.Nil, ErrInvalidCredentials},
		{"B's password on A", h.scope(a), shared, authOtherPasswd, uuid.Nil, ErrInvalidCredentials},
		{"an A-only identifier on B", h.scope(b), onlyA, authPassword, uuid.Nil, ErrInvalidCredentials},
		{"the same A-only identifier on A", h.scope(a), onlyA, authPassword, onlyAUser.ID, nil},
	}
	for _, tc := range cases {
		tokens, err := h.auth.Login(context.Background(), tc.scope, tc.identifier, tc.password)
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.wantErr)
			continue
		}
		if tc.wantErr != nil {
			continue
		}
		if tokens.Principal.UserID != tc.want {
			t.Errorf("%s: logged in as %s, want %s", tc.name, tokens.Principal.UserID, tc.want)
		}
		if tokens.Principal.TenantID == nil || *tokens.Principal.TenantID != mustTenant(t, tc.scope) {
			t.Errorf("%s: session tenant = %v, want the host's tenant", tc.name, tokens.Principal.TenantID)
		}
	}
}

func mustTenant(t *testing.T, scope HostScope) uuid.UUID {
	t.Helper()
	if scope.Platform {
		t.Fatal("a platform scope has no tenant")
	}
	return scope.Tenant.ID
}

func TestPasswordOver72BytesRejected(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)

	tooLong := strings.Repeat("a", 73)
	if _, err := h.auth.Login(context.Background(), h.scope(tenant), *user.Email, tooLong); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("a 73-byte password: err = %v, want ErrInvalidCredentials", err)
	}
	if strings.Contains(h.logs.String(), tooLong) {
		t.Error("the captured logs contain the rejected password")
	}

	// 72 bytes is the boundary bcrypt still uses, so it must stay a usable password.
	exact := strings.Repeat("b", 72)
	boundary := h.userWithEmail(&tenant.ID, model.RoleStudent, "boundary-"+uuid.NewString()[:8]+"@example.com", h.hashed(exact))
	if _, err := h.auth.Login(context.Background(), h.scope(tenant), *boundary.Email, exact); err != nil {
		t.Errorf("a 72-byte password: err = %v, want nil", err)
	}
}

func TestPlatformAdminOnlyOnPlatformHost(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	root := h.user(nil, model.RolePlatformAdmin, authPassword)
	student := h.user(&tenant.ID, model.RoleStudent, authPassword)

	if tokens := h.login(h.platformScope(), *root.Email, authPassword); tokens.Principal.TenantID != nil {
		t.Errorf("platform login tenant = %v, want nil", tokens.Principal.TenantID)
	}
	if tokens := h.login(h.scope(tenant), *student.Email, authPassword); tokens.Principal.TenantID == nil {
		t.Error("tenant login tenant = nil, want the host's tenant")
	}

	cases := []struct {
		name       string
		scope      HostScope
		identifier string
	}{
		{"platform admin on a tenant host", h.scope(tenant), *root.Email},
		{"tenant student on the platform host", h.platformScope(), *student.Email},
	}
	for _, tc := range cases {
		if _, err := h.auth.Login(context.Background(), tc.scope, tc.identifier, authPassword); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%s: err = %v, want ErrInvalidCredentials", tc.name, err)
		}
	}
}

// Only HS256 tokens are accepted, even when another algorithm carries a valid signature.
func TestJWTPinsAlgorithm(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	tokens := h.login(h.scope(tenant), *user.Email, authPassword)

	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); err != nil {
		t.Fatalf("the service's own HS256 token was rejected: %v", err)
	}

	none := jwt.NewWithClaims(jwt.SigningMethodNone, accessClaims{})
	noneToken, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign alg=none: %v", err)
	}

	cases := []struct {
		name  string
		token string
	}{
		{"HS384 with the right key", h.signed(jwt.SigningMethodHS384, tokens.Principal, nil)},
		{"HS512 with the right key", h.signed(jwt.SigningMethodHS512, tokens.Principal, nil)},
		{"alg=none", noneToken},
	}
	for _, tc := range cases {
		if _, err := h.authenticate(h.scope(tenant), tc.token); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("%s: err = %v, want ErrUnauthenticated", tc.name, err)
		}
	}
}

func TestJWTRejectsBadClaims(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	tokens := h.login(h.scope(tenant), *user.Email, authPassword)
	past := h.clock.Now().Add(-time.Hour)

	cases := []struct {
		name   string
		mutate func(*accessClaims)
	}{
		{"a wrong issuer", func(c *accessClaims) { c.Issuer = "someone-else" }},
		{"a wrong audience", func(c *accessClaims) { c.Audience = jwt.ClaimStrings{"another-app"} }},
		{"an expired token", func(c *accessClaims) { c.ExpiresAt = jwt.NewNumericDate(past) }},
		{"no expiry at all", func(c *accessClaims) { c.ExpiresAt = nil }},
		{"a zero subject", func(c *accessClaims) { c.Subject = uuid.Nil.String() }},
		{"a subject that is not a uuid", func(c *accessClaims) { c.Subject = "not-a-uuid" }},
		{"a zero session id", func(c *accessClaims) { c.SessionID = uuid.Nil }},
		{"a subject that is empty", func(c *accessClaims) { c.Subject = "" }},
	}
	for _, tc := range cases {
		if _, err := h.authenticate(h.scope(tenant), h.signed(jwt.SigningMethodHS256, tokens.Principal, tc.mutate)); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("%s: err = %v, want ErrUnauthenticated", tc.name, err)
		}
	}

	// The first signature character carries six real bits; the last carries only four, so flipping it can leave the bytes unchanged.
	tampered := []byte(tokens.AccessToken)
	first := strings.LastIndexByte(tokens.AccessToken, '.') + 1
	if tampered[first] == 'A' {
		tampered[first] = 'B'
	} else {
		tampered[first] = 'A'
	}
	if _, err := h.authenticate(h.scope(tenant), string(tampered)); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("a tampered signature: err = %v, want ErrUnauthenticated", err)
	}
	if _, err := h.authenticate(h.scope(tenant), "not-a-token"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("a malformed token: err = %v, want ErrUnauthenticated", err)
	}
}

// The token, the session and the host must agree; each disagreement is rejected on its own.
func TestMismatchedSubjectSessionTenantRejected(t *testing.T) {
	h := newAuthHarness(t)
	tenant, other := h.tenant(), h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	// The same tenant and the same role, so only the subject check can catch this.
	sibling := h.user(&tenant.ID, model.RoleStudent, authOtherPasswd)
	tokens := h.login(h.scope(tenant), *user.Email, authPassword)

	cases := []struct {
		name   string
		mutate func(*accessClaims)
	}{
		{"sub is another user of the same tenant and role", func(c *accessClaims) { c.Subject = sibling.ID.String() }},
		{"tid is not the session's tenant", func(c *accessClaims) { c.TenantID = other.ID.String() }},
		{"tid is a tenant that does not exist", func(c *accessClaims) { c.TenantID = uuid.NewString() }},
	}
	for _, tc := range cases {
		if _, err := h.authenticate(h.scope(tenant), h.signed(jwt.SigningMethodHS256, tokens.Principal, tc.mutate)); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("%s: err = %v, want ErrUnauthenticated", tc.name, err)
		}
	}

	// Now the session itself names the other tenant: tid matches the session but not the host.
	if _, err := h.pool.Exec(context.Background(), `UPDATE auth_sessions SET tenant_id = $2 WHERE id = $1`, tokens.Principal.SessionID, other.ID); err != nil {
		t.Fatalf("move the session to another tenant: %v", err)
	}
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("a session belonging to another tenant: err = %v, want ErrUnauthenticated", err)
	}
}

func TestForeignTenantTokenRejected(t *testing.T) {
	h := newAuthHarness(t)
	a, b := h.tenant(), h.tenant()
	userB := h.user(&b.ID, model.RoleStudent, authPassword)

	bTokens := h.login(h.scope(b), *userB.Email, authPassword)
	if _, err := h.authenticate(h.scope(b), bTokens.AccessToken); err != nil {
		t.Fatalf("B's own token on B was rejected: %v", err)
	}
	if _, err := h.authenticate(h.scope(a), bTokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("B's token on A's host: err = %v, want ErrUnauthenticated", err)
	}
	if _, err := h.auth.Refresh(context.Background(), h.scope(a), bTokens.RefreshToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("B's refresh token on A's host: err = %v, want ErrUnauthenticated", err)
	}
}

func TestPlatformTokenRejectedOnTenantHostAndReverse(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	root := h.user(nil, model.RolePlatformAdmin, authPassword)
	student := h.user(&tenant.ID, model.RoleStudent, authPassword)

	platformTokens := h.login(h.platformScope(), *root.Email, authPassword)
	tenantTokens := h.login(h.scope(tenant), *student.Email, authPassword)

	if _, err := h.authenticate(h.platformScope(), platformTokens.AccessToken); err != nil {
		t.Fatalf("the platform token on the platform host was rejected: %v", err)
	}
	if _, err := h.authenticate(h.scope(tenant), tenantTokens.AccessToken); err != nil {
		t.Fatalf("the tenant token on its own host was rejected: %v", err)
	}

	if _, err := h.authenticate(h.scope(tenant), platformTokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("the platform token on a tenant host: err = %v, want ErrUnauthenticated", err)
	}
	if _, err := h.authenticate(h.platformScope(), tenantTokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("the tenant token on the platform host: err = %v, want ErrUnauthenticated", err)
	}
}

func TestChangedRoleOrInactiveUserTokenRejected(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	tokens := h.login(h.scope(tenant), *user.Email, authPassword)

	h.setRole(user.ID, model.RoleAdmin)
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("after a role change: err = %v, want ErrUnauthenticated", err)
	}

	h.setRole(user.ID, model.RoleStudent)
	h.setActive(user.ID, false)
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("after a deactivation: err = %v, want ErrUnauthenticated", err)
	}
	if _, err := h.auth.Refresh(context.Background(), h.scope(tenant), tokens.RefreshToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("refresh after a deactivation: err = %v, want ErrUnauthenticated", err)
	}
}

func TestRefreshRotatesTheRefreshToken(t *testing.T) {
	h := newAuthHarness(t)
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	first := h.login(h.scope(tenant), *user.Email, authPassword)

	// A JWT's issued-at has one-second resolution, so move past it before comparing bytes.
	h.clock.Advance(2 * time.Second)

	second, err := h.auth.Refresh(context.Background(), h.scope(tenant), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if second.AccessToken == first.AccessToken {
		t.Error("refresh returned the same access token bytes")
	}
	if second.RefreshToken == "" || second.RefreshToken == first.RefreshToken {
		t.Error("refresh did not rotate the refresh token")
	}
	row := h.sessionRow(second.RefreshToken)
	if !bytes.Equal(row.RefreshHash, hashRefreshToken(second.RefreshToken)) ||
		!bytes.Equal(row.PreviousRefreshHash, hashRefreshToken(first.RefreshToken)) {
		t.Error("the session does not hold the new token as current and the old one as previous")
	}
	if second.Principal.SessionID != first.Principal.SessionID {
		t.Errorf("refresh session = %s, want %s", second.Principal.SessionID, first.Principal.SessionID)
	}
	if second.ExpiresIn != int(AccessTokenTTL/time.Second) {
		t.Errorf("expires_in = %d, want %d", second.ExpiresIn, int(AccessTokenTTL/time.Second))
	}
	if _, err := h.authenticate(h.scope(tenant), second.AccessToken); err != nil {
		t.Errorf("the refreshed access token was rejected: %v", err)
	}

	var rows int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM auth_sessions WHERE user_id = $1`, user.ID).Scan(&rows); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if rows != 1 {
		t.Errorf("sessions = %d, want 1: refresh must not open a new session", rows)
	}
}

// D13 is idle time, not token time: the boundary rejects both sides even though the JWT is still valid.
func TestIdleExpiryRejectsRefreshAndAccess(t *testing.T) {
	h := newAuthHarness(t, WithAuthAccessTTL(time.Hour), WithAuthIdleTTL(30*time.Minute))
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	tokens := h.login(h.scope(tenant), *user.Email, authPassword)

	h.clock.Advance(30 * time.Minute)

	if !h.liveAccessToken(tokens.AccessToken) {
		t.Fatal("the access token is already expired, so this test would not isolate the idle rule")
	}
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("access at the idle boundary: err = %v, want ErrUnauthenticated", err)
	}
	if _, err := h.auth.Refresh(context.Background(), h.scope(tenant), tokens.RefreshToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("refresh at the idle boundary: err = %v, want ErrUnauthenticated", err)
	}

	h.clock.Advance(-time.Second) // one second inside the boundary
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); err != nil {
		t.Errorf("access one second inside the idle window: err = %v, want nil", err)
	}
}

// Refreshing is not activity: the idle deadline must not move.
func TestRefreshDoesNotExtendIdle(t *testing.T) {
	h := newAuthHarness(t, WithAuthAccessTTL(time.Hour), WithAuthIdleTTL(30*time.Minute))
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	tokens := h.login(h.scope(tenant), *user.Email, authPassword)
	start := h.clock.Now()

	h.clock.Advance(20 * time.Minute)
	rotated, err := h.auth.Refresh(context.Background(), h.scope(tenant), tokens.RefreshToken)
	if err != nil {
		t.Fatalf("refresh inside the idle window: %v", err)
	}
	if got := h.sessionRow(tokens.RefreshToken).LastActivityAt; !got.Equal(start) {
		t.Errorf("refresh moved last_activity_at to %s, want it unchanged at %s", got, start)
	}

	// 20m + 11m is past the ORIGINAL 30m deadline, which a refresh must not have pushed out.
	// The rotated token is used, so the refusal is the idle rule and not reuse detection.
	h.clock.Advance(11 * time.Minute)
	if _, err := h.auth.Refresh(context.Background(), h.scope(tenant), rotated.RefreshToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("refresh past the original idle deadline: err = %v, want ErrUnauthenticated", err)
	}
	if h.sessionRow(rotated.RefreshToken).RevokedAt != nil {
		t.Error("an idle refusal revoked the session; only reuse should")
	}
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("access past the original idle deadline: err = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthenticatedRequestExtendsIdleAtMostOncePerMinute(t *testing.T) {
	h := newAuthHarness(t, WithAuthAccessTTL(6*time.Hour))
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	tokens := h.login(h.scope(tenant), *user.Email, authPassword)
	start := h.clock.Now()

	h.clock.Advance(90 * time.Second)
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); err != nil {
		t.Fatalf("authenticate after 90s: %v", err)
	}
	if got := h.sessionRow(tokens.RefreshToken).LastActivityAt; !got.Equal(start.Add(90 * time.Second)) {
		t.Errorf("last_activity_at = %s, want %s", got, start.Add(90*time.Second))
	}

	// Ten seconds later the write window has not reopened, so the row must not move again.
	h.clock.Advance(10 * time.Second)
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); err != nil {
		t.Fatalf("authenticate after 100s: %v", err)
	}
	if got := h.sessionRow(tokens.RefreshToken).LastActivityAt; !got.Equal(start.Add(90 * time.Second)) {
		t.Errorf("last_activity_at = %s, want it unchanged at %s", got, start.Add(90*time.Second))
	}

	h.clock.Advance(100 * time.Second)
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); err != nil {
		t.Fatalf("authenticate after 200s: %v", err)
	}
	if got := h.sessionRow(tokens.RefreshToken).LastActivityAt; !got.Equal(start.Add(200 * time.Second)) {
		t.Errorf("last_activity_at = %s, want %s", got, start.Add(200*time.Second))
	}
}

func TestExpiredAccessThenRefreshThenRequestKeepsSessionAlive(t *testing.T) {
	h := newAuthHarness(t, WithAuthAccessTTL(time.Minute), WithAuthIdleTTL(30*time.Minute))
	tenant := h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	first := h.login(h.scope(tenant), *user.Email, authPassword)

	h.clock.Advance(2 * time.Minute)
	if _, err := h.authenticate(h.scope(tenant), first.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("an expired access token: err = %v, want ErrUnauthenticated", err)
	}

	second, err := h.auth.Refresh(context.Background(), h.scope(tenant), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh after the access token expired: %v", err)
	}
	if _, err := h.authenticate(h.scope(tenant), second.AccessToken); err != nil {
		t.Errorf("the refreshed token was rejected: %v", err)
	}
	if h.sessionRow(first.RefreshToken).RevokedAt != nil {
		t.Error("the session was revoked by the access token expiring")
	}
}

func TestLogoutRevokesImmediately(t *testing.T) {
	h := newAuthHarness(t)
	tenant, other := h.tenant(), h.tenant()
	user := h.user(&tenant.ID, model.RoleStudent, authPassword)
	tokens := h.login(h.scope(tenant), *user.Email, authPassword)

	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); err != nil {
		t.Fatalf("before logout: %v", err)
	}

	// A token for another tenant cannot revoke this session.
	crossTenant := h.login(h.scope(other), *h.user(&other.ID, model.RoleStudent, authOtherPasswd).Email, authOtherPasswd)
	if err := h.auth.Logout(context.Background(), h.scope(other), tokens.RefreshToken); err != nil {
		t.Errorf("logout with a foreign refresh token: %v", err)
	}
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); err != nil {
		t.Errorf("a foreign logout revoked the session: %v", err)
	}

	if err := h.auth.Logout(context.Background(), h.scope(tenant), tokens.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("access right after logout: err = %v, want ErrUnauthenticated", err)
	}
	if _, err := h.auth.Refresh(context.Background(), h.scope(tenant), tokens.RefreshToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("refresh right after logout: err = %v, want ErrUnauthenticated", err)
	}
	if h.sessionRow(crossTenant.RefreshToken).RevokedAt != nil {
		t.Error("the other tenant's session was revoked too")
	}

	// Idempotent: a second logout is not an error and changes nothing.
	if err := h.auth.Logout(context.Background(), h.scope(tenant), tokens.RefreshToken); err != nil {
		t.Errorf("second logout: %v", err)
	}
	if err := h.auth.Logout(context.Background(), h.scope(tenant), ""); err != nil {
		t.Errorf("logout without a token: %v", err)
	}
	if _, err := h.authenticate(h.scope(tenant), tokens.AccessToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("after the second logout: err = %v, want ErrUnauthenticated", err)
	}
}
