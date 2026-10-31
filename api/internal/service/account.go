package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
	"github.com/tarunaeduka/bimbel-tryout/api/internal/repository"
)

var (
	ErrEmailTaken      = errors.New("email already registered")
	ErrPhoneTaken      = errors.New("phone already registered")
	ErrAlreadyVerified = errors.New("email already verified")
	ErrNoEmail         = errors.New("account has no email")
	// ErrInvalidCode covers a wrong, expired, used-up or missing code alike.
	ErrInvalidCode = errors.New("invalid or expired code")
	// ErrInvalidResetToken covers a forged, expired, used, foreign or malformed token alike.
	ErrInvalidResetToken = errors.New("invalid or expired reset token")
)

// Field reasons an InputError reports; the transport turns them into user-facing copy.
const (
	ReasonInvalid       = "invalid"
	ReasonPassword      = "password_rule"
	ReasonWrongPassword = "wrong_password"
	ReasonPhoneRequired = "phone_required"
)

// InputError names each field that failed a format rule.
type InputError struct {
	Fields map[string]string
}

func (e *InputError) Error() string { return "invalid input" }

const (
	// D01: the verification code goes out by email over SMTP and never blocks a login.
	codeDigits      = 6
	codeTTL         = 10 * time.Minute
	codeResendAfter = time.Minute
	maxCodeAttempts = 5
	resetTTL        = 30 * time.Minute
	maxNameRunes    = 100

	TemplateEmailCode     = "email-code"
	TemplatePasswordReset = "password-reset"
)

// WebLinks builds links into the web app from configuration only: never from a request header,
// or a forged Origin would put an attacker's host into a genuine reset email.
type WebLinks struct {
	BaseURL string
}

func (w WebLinks) resetURL(token string) string {
	// A fragment never reaches a server, so the token stays out of access logs and Referer headers.
	return w.BaseURL + "/reset-password#token=" + token
}

type AccountDeps struct {
	DB            *pgxpool.Pool
	Users         *repository.UserRepository
	Sessions      *repository.SessionRepository
	Throttles     *repository.ThrottleRepository
	Notifications *NotificationService
	Auth          *AuthService
	Links         WebLinks
	// Secret keys the code and reset-token HMACs, one derived key each.
	Secret []byte
	Logger *slog.Logger
}

type AccountService struct {
	AccountDeps
	codeKey  []byte
	resetKey []byte
	now      func() time.Time
	cost     int
}

type AccountOption func(*AccountService)

func WithAccountClock(now func() time.Time) AccountOption {
	return func(s *AccountService) { s.now = now }
}

// WithAccountPasswordCost lowers bcrypt's cost so tests do not spend a second per signup.
func WithAccountPasswordCost(cost int) AccountOption {
	return func(s *AccountService) { s.cost = cost }
}

func NewAccountService(deps AccountDeps, opts ...AccountOption) *AccountService {
	s := &AccountService{
		AccountDeps: deps,
		codeKey:     deriveKey(deps.Secret, "email-code-v1"),
		resetKey:    deriveKey(deps.Secret, "password-reset-v1"),
		now:         time.Now,
		cost:        bcrypt.DefaultCost,
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func deriveKey(secret []byte, purpose string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(purpose))
	return mac.Sum(nil)
}

type RegisterInput struct {
	Name     string
	Email    string
	Phone    string
	Password string
}

// Register creates the student and logs them in at once. D01: email is required, a phone is
// optional, and the emailed code only verifies the address; it never gates the account.
func (s *AccountService) Register(ctx context.Context, scope HostScope, in RegisterInput) (Tokens, error) {
	fields := map[string]string{}
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameRunes {
		fields["name"] = ReasonInvalid
	}
	email, ok := NormalizeEmail(in.Email)
	if !ok {
		fields["email"] = ReasonInvalid
	}
	var phone *string
	if strings.TrimSpace(in.Phone) != "" {
		p, ok := NormalizePhone(in.Phone)
		if !ok {
			fields["phone"] = ReasonInvalid
		}
		phone = &p
	}
	if !validPassword(in.Password) {
		fields["password"] = ReasonPassword
	}
	if len(fields) > 0 {
		return Tokens{}, &InputError{Fields: fields}
	}

	tenantID := scope.Tenant.ID
	if err := hitThrottle(ctx, s.Throttles, codeThrottle, throttleKey(codeThrottle, tenantID.String(), email), s.now()); err != nil {
		return Tokens{}, err
	}
	// Checked first so the common case answers before bcrypt; the unique constraints still decide races.
	if err := s.taken(ctx, tenantID, email, ErrEmailTaken); err != nil {
		return Tokens{}, err
	}
	if phone != nil {
		if err := s.taken(ctx, tenantID, *phone, ErrPhoneTaken); err != nil {
			return Tokens{}, err
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), s.cost)
	if err != nil {
		return Tokens{}, fmt.Errorf("hash password: %w", err)
	}
	code, err := newCode()
	if err != nil {
		return Tokens{}, err
	}
	now := s.now()
	var user model.User
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		user, err = s.Users.CreateTx(ctx, tx, model.User{
			TenantID: &tenantID, Name: name, Email: &email, Phone: phone,
			PasswordHash: string(hash), Role: model.RoleStudent,
		})
		switch {
		case errors.Is(err, repository.ErrDuplicateEmail):
			return ErrEmailTaken
		case errors.Is(err, repository.ErrDuplicatePhone):
			return ErrPhoneTaken
		case err != nil:
			return err
		}
		return s.sendCode(ctx, tx, scope, user, code, now)
	})
	if err != nil {
		return Tokens{}, err
	}

	tokens, err := s.Auth.startSession(ctx, user)
	if err != nil {
		return Tokens{}, err
	}
	s.Logger.Info("registered",
		slog.String("user_id", user.ID.String()),
		slog.String("tenant_id", tenantID.String()),
	)
	return tokens, nil
}

// UpdateProfile changes the caller's own name and phone; an empty phone clears it. The unique
// constraint decides a duplicate, so saving the phone the account already holds is not a conflict.
func (s *AccountService) UpdateProfile(ctx context.Context, scope HostScope, p model.Principal, rawName, rawPhone string) (model.Principal, error) {
	fields := map[string]string{}
	name := strings.TrimSpace(rawName)
	if name == "" || utf8.RuneCountInString(name) > maxNameRunes {
		fields["name"] = ReasonInvalid
	}
	var phone *string
	if strings.TrimSpace(rawPhone) != "" {
		normalized, ok := NormalizePhone(rawPhone)
		if !ok {
			fields["phone"] = ReasonInvalid
		}
		phone = &normalized
	} else if p.Email == nil {
		fields["phone"] = ReasonPhoneRequired
	}
	if len(fields) > 0 {
		return model.Principal{}, &InputError{Fields: fields}
	}

	user, err := s.Users.UpdateProfile(ctx, scope.Tenant.ID, p.UserID, name, phone)
	switch {
	case errors.Is(err, repository.ErrDuplicatePhone):
		return model.Principal{}, ErrPhoneTaken
	case errors.Is(err, repository.ErrNotFound):
		return model.Principal{}, ErrUnauthenticated
	case err != nil:
		return model.Principal{}, fmt.Errorf("update profile: %w", err)
	}
	s.Logger.Info("profile updated", slog.String("user_id", user.ID.String()))
	return principalFor(user, p.SessionID), nil
}

// ChangePassword needs the current password and ends every other session of the account, so a
// stolen session does not outlive the change. The caller's own session stays signed in. Wrong
// guesses share the login budget's size, counted under their own key so they never lock the login.
func (s *AccountService) ChangePassword(ctx context.Context, scope HostScope, p model.Principal, current, next string) error {
	if !validPassword(next) {
		return &InputError{Fields: map[string]string{"new_password": ReasonPassword}}
	}
	key := throttleKey(loginThrottle, scopeTenantLabel(scope), "password-change:"+p.UserID.String())
	if err := hitThrottle(ctx, s.Throttles, loginThrottle, key, s.now()); err != nil {
		return err
	}
	user, err := s.Auth.userInScope(ctx, scope, p.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrUnauthenticated
	}
	if err != nil {
		return fmt.Errorf("find user for password change: %w", err)
	}
	wrong := &InputError{Fields: map[string]string{"current_password": ReasonWrongPassword}}
	if len(current) > maxPasswordBytes || s.Auth.compare([]byte(user.PasswordHash), []byte(current)) != nil {
		return wrong
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(next), s.cost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := s.now()
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		replaced, err := s.Users.SetPasswordTx(ctx, tx, user.TenantID, user.ID, user.PasswordHash, string(hash))
		if err != nil {
			return err
		}
		if !replaced {
			// A racing change already moved the hash this request verified against.
			return wrong
		}
		return s.Sessions.RevokeOthersForUserTx(ctx, tx, user.ID, p.SessionID, now)
	})
	if err != nil {
		return err
	}
	if err := s.Throttles.Clear(ctx, key, loginThrottle.window, now); err != nil {
		s.Logger.Error("clear password change throttle", slog.String("error", err.Error()))
	}
	s.Logger.Info("password changed", slog.String("user_id", user.ID.String()))
	return nil
}

// ResendEmailCode replaces the caller's code; it shares the per-email budget with Register.
func (s *AccountService) ResendEmailCode(ctx context.Context, scope HostScope, p model.Principal) error {
	if p.Email == nil {
		return ErrNoEmail
	}
	if err := hitThrottle(ctx, s.Throttles, codeThrottle, throttleKey(codeThrottle, scopeTenantLabel(scope), *p.Email), s.now()); err != nil {
		return err
	}
	code, err := newCode()
	if err != nil {
		return err
	}
	now := s.now()
	return s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.Users.FindEmailCodeForUpdateTx(ctx, tx, p.TenantID, p.UserID)
		if err != nil {
			return err
		}
		if current.VerifiedAt != nil {
			return ErrAlreadyVerified
		}
		if current.ExpiresAt != nil {
			sentAt := current.ExpiresAt.Add(-codeTTL)
			if wait := sentAt.Add(codeResendAfter).Sub(now); wait > 0 {
				return &ThrottleError{RetryAfter: wait}
			}
		}
		user := model.User{ID: p.UserID, TenantID: p.TenantID, Name: p.Name, Email: p.Email}
		return s.sendCode(ctx, tx, scope, user, code, now)
	})
}

// VerifyEmail checks the caller's code. Verifying twice is not an error.
func (s *AccountService) VerifyEmail(ctx context.Context, p model.Principal, code string) error {
	if !validCodeShape(code) {
		return ErrInvalidCode
	}
	now := s.now()
	var wrong bool
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.Users.FindEmailCodeForUpdateTx(ctx, tx, p.TenantID, p.UserID)
		if err != nil {
			return err
		}
		if current.VerifiedAt != nil {
			return nil
		}
		if current.Hash == nil || current.ExpiresAt == nil || !now.Before(*current.ExpiresAt) || current.Attempts >= maxCodeAttempts {
			return ErrInvalidCode
		}
		if !hmac.Equal(s.codeHash(p.UserID, code), current.Hash) {
			// The wrong guess is committed, so it counts even though the request fails.
			wrong = true
			return s.Users.CountEmailCodeAttemptTx(ctx, tx, p.TenantID, p.UserID)
		}
		return s.Users.MarkEmailVerifiedTx(ctx, tx, p.TenantID, p.UserID, now)
	})
	if err != nil {
		return err
	}
	if wrong {
		return ErrInvalidCode
	}
	s.Logger.Info("email verified", slog.String("user_id", p.UserID.String()))
	return nil
}

// ForgotPassword answers nil whether or not the email belongs to anyone on this host.
func (s *AccountService) ForgotPassword(ctx context.Context, scope HostScope, rawEmail string) error {
	email, ok := NormalizeEmail(rawEmail)
	if !ok {
		return &InputError{Fields: map[string]string{"email": ReasonInvalid}}
	}
	if err := hitThrottle(ctx, s.Throttles, forgotThrottle, throttleKey(forgotThrottle, scopeTenantLabel(scope), email), s.now()); err != nil {
		return err
	}
	user, err := s.Auth.findInScope(ctx, scope, email)
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find user for reset: %w", err)
	}
	if !user.IsActive || user.Email == nil {
		return nil
	}

	expires := s.now().Add(resetTTL)
	token := s.resetToken(user, expires)
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		return s.Notifications.Enqueue(ctx, tx, NewNotification{
			Scope:     Scope{TenantID: user.TenantID},
			EventKey:  "password-reset:" + uuid.NewString(),
			Channel:   string(model.ChannelEmail),
			Recipient: *user.Email,
			Template:  TemplatePasswordReset,
			Params: map[string]string{
				"tenant_name": tenantName(scope), "name": user.Name, "minutes": minutes(resetTTL),
			},
			SecretParams: map[string]string{"reset_url": s.Links.resetURL(token)},
			ExpiresAt:    &expires,
		})
	})
	if err != nil {
		return err
	}
	s.Logger.Info("password reset sent",
		slog.String("user_id", user.ID.String()),
		slog.String("tenant_id", tenantLabel(user.TenantID)),
	)
	return nil
}

// ResetPassword sets the password and ends every session the old password opened. The token is
// bound to the password hash it was issued against, so once that changes the link is dead.
func (s *AccountService) ResetPassword(ctx context.Context, scope HostScope, token, password string) error {
	if !validPassword(password) {
		return &InputError{Fields: map[string]string{"password": ReasonPassword}}
	}
	userID, expires, mac, ok := parseResetToken(token)
	if !ok || !s.now().Before(expires) {
		return ErrInvalidResetToken
	}
	user, err := s.Auth.userInScope(ctx, scope, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("find user for reset: %w", err)
	}
	if !user.IsActive || !hmac.Equal(mac, s.resetMAC(user, expires)) {
		return ErrInvalidResetToken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := s.now()
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		replaced, err := s.Users.ReplacePasswordTx(ctx, tx, user.TenantID, user.ID, user.PasswordHash, string(hash), now)
		if err != nil {
			return err
		}
		if !replaced {
			// A racing reset spent the link first.
			return ErrInvalidResetToken
		}
		return s.Sessions.RevokeAllForUserTx(ctx, tx, user.ID, now)
	})
	if err != nil {
		return err
	}
	// The owner just proved control of the inbox, so earlier failed logins no longer count against them.
	for _, id := range []*string{user.Email, user.Phone} {
		if id == nil {
			continue
		}
		if err := s.Throttles.Clear(ctx, throttleKey(loginThrottle, scopeTenantLabel(scope), *id), loginThrottle.window, now); err != nil {
			s.Logger.Error("clear login throttle after reset", slog.String("error", err.Error()))
		}
	}
	s.Logger.Info("password reset",
		slog.String("user_id", user.ID.String()),
		slog.String("tenant_id", tenantLabel(user.TenantID)),
	)
	return nil
}

// resetToken is base64url(user id ‖ expiry) "." base64url(HMAC of that and the current password hash).
func (s *AccountService) resetToken(user model.User, expires time.Time) string {
	payload := resetPayload(user.ID, expires)
	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(s.resetMAC(user, expires))
}

func (s *AccountService) resetMAC(user model.User, expires time.Time) []byte {
	mac := hmac.New(sha256.New, s.resetKey)
	mac.Write(resetPayload(user.ID, expires))
	mac.Write([]byte(user.PasswordHash))
	return mac.Sum(nil)
}

func resetPayload(userID uuid.UUID, expires time.Time) []byte {
	payload := make([]byte, 0, 24)
	payload = append(payload, userID[:]...)
	return binary.BigEndian.AppendUint64(payload, uint64(expires.Unix()))
}

func parseResetToken(token string) (uuid.UUID, time.Time, []byte, bool) {
	rawPayload, rawMAC, found := strings.Cut(token, ".")
	if !found {
		return uuid.Nil, time.Time{}, nil, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(rawPayload)
	if err != nil || len(payload) != 24 {
		return uuid.Nil, time.Time{}, nil, false
	}
	mac, err := base64.RawURLEncoding.DecodeString(rawMAC)
	if err != nil || len(mac) != sha256.Size {
		return uuid.Nil, time.Time{}, nil, false
	}
	userID, err := uuid.FromBytes(payload[:16])
	if err != nil {
		return uuid.Nil, time.Time{}, nil, false
	}
	expires := time.Unix(int64(binary.BigEndian.Uint64(payload[16:])), 0)
	return userID, expires, mac, true
}

// taken reports an identifier an existing account already holds in this tenant.
func (s *AccountService) taken(ctx context.Context, tenantID uuid.UUID, identifier string, taken error) error {
	_, err := s.Users.FindByIdentifier(ctx, tenantID, identifier)
	if err == nil {
		return taken
	}
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("check identifier: %w", err)
}

// sendCode stores the new code's HMAC and queues the email in the same transaction.
func (s *AccountService) sendCode(ctx context.Context, tx pgx.Tx, scope HostScope, user model.User, code string, now time.Time) error {
	expires := now.Add(codeTTL)
	if err := s.Users.SetEmailCodeTx(ctx, tx, user.TenantID, user.ID, s.codeHash(user.ID, code), expires); err != nil {
		return err
	}
	return s.Notifications.Enqueue(ctx, tx, NewNotification{
		Scope:     Scope{TenantID: user.TenantID},
		EventKey:  "email-code:" + uuid.NewString(),
		Channel:   string(model.ChannelEmail),
		Recipient: *user.Email,
		Template:  TemplateEmailCode,
		Params: map[string]string{
			"tenant_name": tenantName(scope), "name": user.Name, "minutes": minutes(codeTTL),
		},
		SecretParams: map[string]string{"code": code},
		// A code that reaches the inbox after it stopped working is worse than none.
		ExpiresAt: &expires,
	})
}

func (s *AccountService) codeHash(userID uuid.UUID, code string) []byte {
	mac := hmac.New(sha256.New, s.codeKey)
	mac.Write(userID[:])
	mac.Write([]byte(code))
	return mac.Sum(nil)
}

// inTx commits only when fn succeeds; a failed guess is committed by fn returning nil.
func (s *AccountService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// newCode is uniform over 000000–999999.
func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%0*d", codeDigits, n.Int64()), nil
}

func validCodeShape(code string) bool {
	if len(code) != codeDigits {
		return false
	}
	return strings.Trim(code, "0123456789") == ""
}

func tenantName(scope HostScope) string {
	return scope.Tenant.Name
}

func minutes(d time.Duration) string { return strconv.Itoa(int(d / time.Minute)) }
