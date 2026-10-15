package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	// One answer for every failed login, so a caller cannot probe which identifiers exist.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUnauthenticated covers every unusable token or session, for the same reason.
	ErrUnauthenticated = errors.New("unauthenticated")
)

const (
	// AccessTokenTTL and IdleTTL are D13: a short access token carried by an idle-limited session.
	AccessTokenTTL = 15 * time.Minute
	IdleTTL        = 3 * time.Hour

	refreshTokenBytes = 32
	// refreshReuseGrace lets two tabs that refresh at once both succeed; a replay after it is theft.
	refreshReuseGrace = 30 * time.Second
	// activityWriteGap bounds how often one session may write last_activity_at.
	activityWriteGap = time.Minute

	accessTokenIssuer   = "taruna-api"
	accessTokenAudience = "taruna-web"
	accessTokenLeeway   = 5 * time.Second

	// maxPasswordBytes is bcrypt's own limit: a longer input is not a password a person typed.
	maxPasswordBytes = 72

	// A cost-10 bcrypt hash nobody claims, so an unknown or inactive account burns the same time as a wrong password.
	dummyPasswordHash = "$2a$10$NBUsWPWnoOj0a36NBJg5E.xOKv.6Z59/EuF4QVI7R0oCGkLjrfq82"
)

// The refresh token is returned only so the transport can set the HttpOnly cookie; no body or log carries it.
// It is empty when the browser already holds the current one and the cookie must stay as it is.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	Principal    model.Principal
}

type AuthService struct {
	users     *repository.UserRepository
	sessions  *repository.SessionRepository
	throttles *repository.ThrottleRepository
	secret    []byte
	now       func() time.Time
	compare   func(hash, password []byte) error
	logger    *slog.Logger

	accessTTL time.Duration
	idleTTL   time.Duration
}

type AuthOption func(*AuthService)

// WithAuthClock replaces the clock; every session rule is time-based, so tests move time instead of waiting.
func WithAuthClock(now func() time.Time) AuthOption {
	return func(s *AuthService) { s.now = now }
}

// WithAuthAccessTTL overrides the access-token lifetime so a test can expire one immediately.
func WithAuthAccessTTL(d time.Duration) AuthOption {
	return func(s *AuthService) { s.accessTTL = d }
}

// WithAuthIdleTTL overrides the D13 idle lifetime so a test can cross it without waiting three hours.
func WithAuthIdleTTL(d time.Duration) AuthOption {
	return func(s *AuthService) { s.idleTTL = d }
}

// WithAuthPasswordCompare replaces bcrypt; a test uses it to observe that unknown accounts still pay for a compare.
func WithAuthPasswordCompare(fn func(hash, password []byte) error) AuthOption {
	return func(s *AuthService) { s.compare = fn }
}

func NewAuthService(users *repository.UserRepository, sessions *repository.SessionRepository, throttles *repository.ThrottleRepository, secret []byte, logger *slog.Logger, opts ...AuthOption) *AuthService {
	s := &AuthService{
		users:     users,
		sessions:  sessions,
		throttles: throttles,
		secret:    secret,
		now:       time.Now,
		compare:   bcrypt.CompareHashAndPassword,
		logger:    logger,
		accessTTL: AccessTokenTTL,
		idleTTL:   IdleTTL,
	}
	if s.logger == nil {
		s.logger = slog.Default()
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// The identifier resolves inside the host's scope only, so one email in two tenants is two people.
// Attempts are counted before the password is checked, so concurrent guesses cannot slip past the limit.
func (s *AuthService) Login(ctx context.Context, scope HostScope, identifier, password string) (Tokens, error) {
	if identifier == "" || password == "" || len(password) > maxPasswordBytes {
		s.logger.Warn("login failed", slog.String("tenant_id", scopeTenantLabel(scope)))
		return Tokens{}, ErrInvalidCredentials
	}
	identifier, ok := NormalizeIdentifier(identifier)
	if !ok {
		// No account can hold this identifier, which must still cost what an unknown account costs.
		s.dummyCompare(password)
		s.logger.Warn("login failed", slog.String("tenant_id", scopeTenantLabel(scope)))
		return Tokens{}, ErrInvalidCredentials
	}
	throttleKey := throttleKey(loginThrottle, scopeTenantLabel(scope), identifier)
	if err := hitThrottle(ctx, s.throttles, loginThrottle, throttleKey, s.now()); err != nil {
		var limited *ThrottleError
		if errors.As(err, &limited) {
			s.logger.Warn("login throttled", slog.String("tenant_id", scopeTenantLabel(scope)))
		}
		return Tokens{}, err
	}

	user, err := s.findInScope(ctx, scope, identifier)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return Tokens{}, fmt.Errorf("find user for login: %w", err)
		}
		s.dummyCompare(password)
		s.logger.Warn("login failed", slog.String("tenant_id", scopeTenantLabel(scope)))
		return Tokens{}, ErrInvalidCredentials
	}
	if !user.IsActive {
		s.dummyCompare(password)
		s.logger.Warn("login failed", slog.String("tenant_id", scopeTenantLabel(scope)))
		return Tokens{}, ErrInvalidCredentials
	}
	if err := s.compare([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.logger.Warn("login failed", slog.String("tenant_id", scopeTenantLabel(scope)))
		return Tokens{}, ErrInvalidCredentials
	}

	tokens, err := s.startSession(ctx, user)
	if err != nil {
		return Tokens{}, err
	}
	// A success forgives earlier typos; a failure to clear only leaves the window to run out.
	if err := s.throttles.Clear(ctx, throttleKey); err != nil {
		s.logger.Error("clear login throttle", slog.String("error", err.Error()))
	}
	s.logger.Info("login",
		slog.String("user_id", user.ID.String()),
		slog.String("session_id", tokens.Principal.SessionID.String()),
		slog.String("tenant_id", tenantLabel(user.TenantID)),
	)
	return tokens, nil
}

// Refresh deliberately does not move last_activity_at: reloading a page must not extend the idle deadline.
// Every refresh rotates the token. The replaced token still works for refreshReuseGrace so racing tabs
// both succeed; replayed after that, it can only be a copy, and the whole session is revoked.
func (s *AuthService) Refresh(ctx context.Context, scope HostScope, refreshToken string) (Tokens, error) {
	if refreshToken == "" {
		return Tokens{}, ErrUnauthenticated
	}
	presented := hashRefreshToken(refreshToken)
	session, err := s.sessions.FindByRefreshHash(ctx, presented)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return Tokens{}, ErrUnauthenticated
		}
		return Tokens{}, s.deny(err)
	}
	if session.RevokedAt != nil || !s.live(session) || !sameTenant(session.TenantID, scopeTenant(scope)) {
		return Tokens{}, ErrUnauthenticated
	}

	user, err := s.userInScope(ctx, scope, session.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return Tokens{}, ErrUnauthenticated
		}
		return Tokens{}, s.deny(err)
	}
	if !user.IsActive {
		return Tokens{}, ErrUnauthenticated
	}

	next, err := s.rotate(ctx, session, presented)
	if err != nil {
		return Tokens{}, err
	}
	tokens, err := s.issue(user, session.ID, next)
	if err != nil {
		return Tokens{}, err
	}
	s.logger.Info("refresh",
		slog.String("user_id", user.ID.String()),
		slog.String("session_id", session.ID.String()),
		slog.String("tenant_id", tenantLabel(user.TenantID)),
	)
	return tokens, nil
}

// An unknown, foreign or already-revoked token is indistinguishable from success.
func (s *AuthService) Logout(ctx context.Context, scope HostScope, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	revoked, err := s.sessions.RevokeByRefreshHash(ctx, hashRefreshToken(refreshToken), scopeTenant(scope), s.now())
	if err != nil {
		return err
	}
	if revoked {
		s.logger.Info("logout", slog.String("tenant_id", scopeTenantLabel(scope)))
	}
	return nil
}

// Rebuilt from live session and user rows so a revoke, role change or deactivation bites on the next request.
func (s *AuthService) Authenticate(ctx context.Context, scope HostScope, rawAccessToken string) (model.Principal, error) {
	claims, err := s.parseAccessToken(rawAccessToken)
	if err != nil {
		return model.Principal{}, ErrUnauthenticated
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil || userID == uuid.Nil || claims.SessionID == uuid.Nil {
		return model.Principal{}, ErrUnauthenticated
	}

	session, err := s.sessions.FindByID(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Principal{}, ErrUnauthenticated
		}
		return model.Principal{}, s.deny(err)
	}
	if session.UserID != userID || session.RevokedAt != nil || !s.live(session) {
		return model.Principal{}, ErrUnauthenticated
	}

	// The host scope, the session and the token must all name the same tenant; "" is the platform.
	hostTenant := scopeTenant(scope)
	if !sameTenant(session.TenantID, hostTenant) || !claimMatchesTenant(claims.TenantID, hostTenant) {
		return model.Principal{}, ErrUnauthenticated
	}

	user, err := s.userInScope(ctx, scope, session.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Principal{}, ErrUnauthenticated
		}
		return model.Principal{}, s.deny(err)
	}
	if !user.IsActive || user.Role != claims.Role {
		return model.Principal{}, ErrUnauthenticated
	}

	// One conditional write per activity window; being a no-op is the expected outcome.
	now := s.now()
	if err := s.sessions.Touch(ctx, session.ID, now.Add(-activityWriteGap), now); err != nil {
		return model.Principal{}, s.deny(err)
	}
	return principalFor(user, session.ID), nil
}

// rotate returns the token the browser should hold next, or "" when it already holds the current one.
func (s *AuthService) rotate(ctx context.Context, session model.Session, presented []byte) (string, error) {
	now := s.now()
	if !bytes.Equal(session.RefreshHash, presented) {
		if session.RotatedAt != nil && now.Sub(*session.RotatedAt) <= refreshReuseGrace {
			return "", nil
		}
		if err := s.sessions.Revoke(ctx, session.ID, now); err != nil {
			return "", s.deny(err)
		}
		s.logger.Warn("refresh token reused; session revoked",
			slog.String("session_id", session.ID.String()),
			slog.String("tenant_id", tenantLabel(session.TenantID)),
		)
		return "", ErrUnauthenticated
	}

	next, err := newRefreshToken()
	if err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	rotated, err := s.sessions.Rotate(ctx, session.ID, presented, hashRefreshToken(next), now)
	if err != nil {
		return "", s.deny(err)
	}
	if !rotated {
		// A concurrent refresh rotated first and its response carries the new cookie.
		return "", nil
	}
	return next, nil
}

func (s *AuthService) startSession(ctx context.Context, user model.User) (Tokens, error) {
	now := s.now()
	refreshToken, err := newRefreshToken()
	if err != nil {
		return Tokens{}, fmt.Errorf("generate refresh token: %w", err)
	}
	session, err := s.sessions.Create(ctx, model.Session{
		UserID:         user.ID,
		TenantID:       user.TenantID,
		RefreshHash:    hashRefreshToken(refreshToken),
		CreatedAt:      now,
		LastActivityAt: now,
	})
	if err != nil {
		return Tokens{}, fmt.Errorf("create session: %w", err)
	}
	return s.issue(user, session.ID, refreshToken)
}

func (s *AuthService) issue(user model.User, sessionID uuid.UUID, refreshToken string) (Tokens, error) {
	principal := principalFor(user, sessionID)
	accessToken, err := s.mintAccessToken(principal)
	if err != nil {
		return Tokens{}, fmt.Errorf("sign access token: %w", err)
	}
	return Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.accessTTL / time.Second),
		Principal:    principal,
	}, nil
}

func (s *AuthService) findInScope(ctx context.Context, scope HostScope, identifier string) (model.User, error) {
	if scope.Platform {
		return s.users.FindPlatformByIdentifier(ctx, identifier)
	}
	return s.users.FindByIdentifier(ctx, scope.Tenant.ID, identifier)
}

func (s *AuthService) userInScope(ctx context.Context, scope HostScope, userID uuid.UUID) (model.User, error) {
	if scope.Platform {
		return s.users.FindPlatformByID(ctx, userID)
	}
	return s.users.FindByID(ctx, scope.Tenant.ID, userID)
}

// live reports whether the idle rule still holds; the boundary itself is expired.
func (s *AuthService) live(session model.Session) bool {
	return s.now().Sub(session.LastActivityAt) < s.idleTTL
}

// dummyCompare spends a real bcrypt comparison whose result is thrown away.
func (s *AuthService) dummyCompare(password string) {
	_ = s.compare([]byte(dummyPasswordHash), []byte(password))
}

// deny keeps the client answer uniform while the operator still gets the real cause in the log.
func (s *AuthService) deny(err error) error {
	s.logger.Error("session lookup failed", slog.String("error", err.Error()))
	return ErrUnauthenticated
}

// accessClaims is the wire shape of the access token. tid is "" on the platform host.
type accessClaims struct {
	SessionID uuid.UUID  `json:"sid"`
	TenantID  string     `json:"tid"`
	Role      model.Role `json:"role"`
	jwt.RegisteredClaims
}

func (s *AuthService) mintAccessToken(p model.Principal) (string, error) {
	now := s.now()
	// The parse side only checks iss/aud, so issuance has to set them.
	claims := accessClaims{
		SessionID: p.SessionID,
		TenantID:  tenantLabel(p.TenantID),
		Role:      p.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    accessTokenIssuer,
			Subject:   p.UserID.String(),
			Audience:  jwt.ClaimStrings{accessTokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *AuthService) parseAccessToken(raw string) (accessClaims, error) {
	var claims accessClaims
	_, err := jwt.ParseWithClaims(raw, &claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(accessTokenIssuer),
		jwt.WithAudience(accessTokenAudience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(accessTokenLeeway),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		return accessClaims{}, err
	}
	return claims, nil
}

// newRefreshToken is the only place a refresh token exists in plaintext.
func newRefreshToken() (string, error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// hashRefreshToken is what the database stores: SHA-256 of the token the browser holds.
func hashRefreshToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func principalFor(u model.User, sessionID uuid.UUID) model.Principal {
	return model.Principal{
		UserID:    u.ID,
		SessionID: sessionID,
		TenantID:  u.TenantID,
		Name:      u.Name,
		Email:     u.Email,
		Phone:     u.Phone,
		Role:      u.Role,
	}
}

// scopeTenant is the host scope as the nullable tenant the database uses: nil means the platform.
func scopeTenant(scope HostScope) *uuid.UUID {
	if scope.Platform {
		return nil
	}
	id := scope.Tenant.ID
	return &id
}

func scopeTenantLabel(scope HostScope) string { return tenantLabel(scopeTenant(scope)) }

func tenantLabel(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func sameTenant(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// claimMatchesTenant requires the token's tenant claim to be exactly the host's scope.
func claimMatchesTenant(claim string, scopeTenantID *uuid.UUID) bool {
	if claim == "" {
		return scopeTenantID == nil
	}
	id, err := uuid.Parse(claim)
	if err != nil {
		return false
	}
	return scopeTenantID != nil && id == *scopeTenantID
}

// ThrottleError is a refusal the client may retry once RetryAfter has passed.
type ThrottleError struct {
	RetryAfter time.Duration
}

func (e *ThrottleError) Error() string { return "too many attempts" }

type throttleRule struct {
	name   string
	limit  int
	window time.Duration
}

// Budgets sit well above what a person mistypes and well below what a guesser needs.
var (
	loginThrottle  = throttleRule{name: "login", limit: 10, window: 15 * time.Minute}
	codeThrottle   = throttleRule{name: "registration-code", limit: 5, window: time.Hour}
	forgotThrottle = throttleRule{name: "password-forgot", limit: 3, window: 15 * time.Minute}
)

// throttleKey is per rule, scope and subject; hashing keeps the identifier out of the table.
func throttleKey(rule throttleRule, scope, subject string) []byte {
	sum := sha256.Sum256([]byte(rule.name + "\x00" + scope + "\x00" + subject))
	return sum[:]
}

func hitThrottle(ctx context.Context, repo *repository.ThrottleRepository, rule throttleRule, key []byte, now time.Time) error {
	hits, start, err := repo.Hit(ctx, key, rule.window, now)
	if err != nil {
		return err
	}
	if hits > rule.limit {
		return &ThrottleError{RetryAfter: start.Add(rule.window).Sub(now)}
	}
	return nil
}
