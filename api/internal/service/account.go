package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
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
	// ErrRegistrationClosed is the platform host: Platform Admins are provisioned, never self-registered.
	ErrRegistrationClosed = errors.New("registration is not offered on this host")
	ErrEmailTaken         = errors.New("email already registered")
	ErrPhoneTaken         = errors.New("phone already registered")
	// ErrInvalidCode covers a wrong, expired, exhausted or unknown code alike.
	ErrInvalidCode = errors.New("invalid or expired code")
	// ErrInvalidResetToken covers an unknown, used, expired or foreign token alike.
	ErrInvalidResetToken = errors.New("invalid or expired reset token")
)

// Field reasons an InputError reports; the transport turns them into user-facing copy.
const (
	ReasonInvalid  = "invalid"
	ReasonPassword = "password_rule"
)

// InputError names each field that failed a format rule.
type InputError struct {
	Fields map[string]string
}

func (e *InputError) Error() string { return "invalid input" }

const (
	// D01: the registration code goes out by email over SMTP.
	codeDigits      = 6
	codeTTL         = 10 * time.Minute
	codeResendAfter = time.Minute
	maxCodeAttempts = 5
	resetTTL        = 30 * time.Minute
	maxNameRunes    = 100

	TemplateRegistrationCode = "registration-code"
	TemplatePasswordReset    = "password-reset"
)

// WebLinks builds links into the web app from configuration only: never from a request header,
// or a forged Origin would put an attacker's host into a genuine reset email.
type WebLinks struct {
	Scheme     string
	BaseDomain string
	Port       string
}

func (w WebLinks) resetURL(scope HostScope, token string) string {
	label := platformLabel
	if !scope.Platform {
		label = scope.Tenant.Slug
	}
	host := label + "." + w.BaseDomain
	if w.Port != "" {
		host = net.JoinHostPort(host, w.Port)
	}
	// A fragment never reaches a server, so the token stays out of access logs and Referer headers.
	return w.Scheme + "://" + host + "/reset-password#token=" + token
}

type AccountDeps struct {
	DB            *pgxpool.Pool
	Users         *repository.UserRepository
	Accounts      *repository.AccountRepository
	Sessions      *repository.SessionRepository
	Throttles     *repository.ThrottleRepository
	Notifications *NotificationService
	Auth          *AuthService
	Links         WebLinks
	// Secret keys the code HMAC; a database dump alone cannot test a million codes against it.
	Secret []byte
	Logger *slog.Logger
}

type AccountService struct {
	AccountDeps
	codeKey []byte
	now     func() time.Time
	cost    int
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
	mac := hmac.New(sha256.New, deps.Secret)
	mac.Write([]byte("registration-code-v1"))
	s := &AccountService{AccountDeps: deps, codeKey: mac.Sum(nil), now: time.Now, cost: bcrypt.DefaultCost}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type RegisterInput struct {
	Name     string
	Email    string
	Phone    string
	Password string
}

// CodeSent is the same for a first signup and a resend, known or not, so it reveals nothing.
type CodeSent struct {
	Email       string
	ExpiresIn   time.Duration
	ResendAfter time.Duration
}

// Register keeps the signup pending and emails a code. D01: email is required, a phone is optional.
func (s *AccountService) Register(ctx context.Context, scope HostScope, in RegisterInput) (CodeSent, error) {
	if scope.Platform {
		return CodeSent{}, ErrRegistrationClosed
	}
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
		return CodeSent{}, &InputError{Fields: fields}
	}

	tenantID := scope.Tenant.ID
	if err := hitThrottle(ctx, s.Throttles, codeThrottle, throttleKey(codeThrottle, tenantID.String(), email), s.now()); err != nil {
		return CodeSent{}, err
	}
	if err := s.taken(ctx, tenantID, email, ErrEmailTaken); err != nil {
		return CodeSent{}, err
	}
	if phone != nil {
		if err := s.taken(ctx, tenantID, *phone, ErrPhoneTaken); err != nil {
			return CodeSent{}, err
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), s.cost)
	if err != nil {
		return CodeSent{}, fmt.Errorf("hash password: %w", err)
	}
	code, codeID, err := newCode()
	if err != nil {
		return CodeSent{}, err
	}
	now := s.now()
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		reg, err := s.Accounts.UpsertRegistrationTx(ctx, tx, model.Registration{
			TenantID: tenantID, Email: email, Phone: phone, Name: name, PasswordHash: string(hash),
			CodeID: codeID, CodeHash: s.codeHash(codeID, code), ExpiresAt: now.Add(codeTTL), SentAt: now,
		})
		if err != nil {
			return err
		}
		return s.sendCode(ctx, tx, scope, reg, code)
	})
	if err != nil {
		return CodeSent{}, err
	}
	s.Logger.Info("registration code sent", slog.String("tenant_id", tenantID.String()))
	return codeSent(email), nil
}

// ResendCode answers the same for an unknown email as for a pending one.
func (s *AccountService) ResendCode(ctx context.Context, scope HostScope, rawEmail string) (CodeSent, error) {
	if scope.Platform {
		return CodeSent{}, ErrRegistrationClosed
	}
	email, ok := NormalizeEmail(rawEmail)
	if !ok {
		return CodeSent{}, &InputError{Fields: map[string]string{"email": ReasonInvalid}}
	}
	tenantID := scope.Tenant.ID
	if err := hitThrottle(ctx, s.Throttles, codeThrottle, throttleKey(codeThrottle, tenantID.String(), email), s.now()); err != nil {
		return CodeSent{}, err
	}
	code, codeID, err := newCode()
	if err != nil {
		return CodeSent{}, err
	}
	now := s.now()
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		reg, err := s.Accounts.FindRegistrationForUpdateTx(ctx, tx, tenantID, email)
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if wait := reg.SentAt.Add(codeResendAfter).Sub(now); wait > 0 {
			return &ThrottleError{RetryAfter: wait}
		}
		reg.CodeID, reg.CodeHash, reg.ExpiresAt, reg.SentAt = codeID, s.codeHash(codeID, code), now.Add(codeTTL), now
		if err := s.Accounts.ReplaceCodeTx(ctx, tx, tenantID, reg.ID, reg.CodeID, reg.CodeHash, reg.ExpiresAt, reg.SentAt); err != nil {
			return err
		}
		return s.sendCode(ctx, tx, scope, reg, code)
	})
	if err != nil {
		return CodeSent{}, err
	}
	return codeSent(email), nil
}

// VerifyCode turns the pending signup into a student account and opens its first session.
func (s *AccountService) VerifyCode(ctx context.Context, scope HostScope, rawEmail, code string) (Tokens, error) {
	if scope.Platform {
		return Tokens{}, ErrRegistrationClosed
	}
	email, ok := NormalizeEmail(rawEmail)
	if !ok || !validCodeShape(code) {
		return Tokens{}, ErrInvalidCode
	}
	tenantID := scope.Tenant.ID
	now := s.now()

	var user model.User
	var wrong bool
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		reg, err := s.Accounts.FindRegistrationForUpdateTx(ctx, tx, tenantID, email)
		if errors.Is(err, repository.ErrNotFound) {
			return ErrInvalidCode
		}
		if err != nil {
			return err
		}
		if !now.Before(reg.ExpiresAt) || reg.Attempts >= maxCodeAttempts {
			return ErrInvalidCode
		}
		if !hmac.Equal(s.codeHash(reg.CodeID, code), reg.CodeHash) {
			// The wrong guess is committed, so it counts even though the request fails.
			wrong = true
			return s.Accounts.CountAttemptTx(ctx, tx, tenantID, reg.ID)
		}
		user, err = s.Users.CreateTx(ctx, tx, model.User{
			TenantID: &tenantID, Name: reg.Name, Email: &reg.Email, Phone: reg.Phone,
			PasswordHash: reg.PasswordHash, Role: model.RoleStudent,
		})
		switch {
		case errors.Is(err, repository.ErrDuplicateEmail):
			return ErrEmailTaken
		case errors.Is(err, repository.ErrDuplicatePhone):
			return ErrPhoneTaken
		case err != nil:
			return err
		}
		return s.Accounts.DeleteRegistrationTx(ctx, tx, tenantID, reg.ID)
	})
	if err != nil {
		return Tokens{}, err
	}
	if wrong {
		return Tokens{}, ErrInvalidCode
	}

	tokens, err := s.Auth.startSession(ctx, user)
	if err != nil {
		return Tokens{}, err
	}
	s.Logger.Info("registration verified",
		slog.String("user_id", user.ID.String()),
		slog.String("tenant_id", tenantID.String()),
	)
	return tokens, nil
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

	token, err := newRefreshToken()
	if err != nil {
		return fmt.Errorf("generate reset token: %w", err)
	}
	now := s.now()
	expires := now.Add(resetTTL)
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.Accounts.CreateResetTx(ctx, tx, model.PasswordReset{
			UserID: user.ID, TenantID: user.TenantID, TokenHash: hashRefreshToken(token), ExpiresAt: expires, CreatedAt: now,
		}); err != nil {
			return err
		}
		return s.Notifications.Enqueue(ctx, tx, NewNotification{
			Scope:     Scope{TenantID: user.TenantID},
			EventKey:  "password-reset:" + uuid.NewString(),
			Channel:   string(model.ChannelEmail),
			Recipient: *user.Email,
			Template:  TemplatePasswordReset,
			Params: map[string]string{
				"tenant_name": tenantName(scope), "name": user.Name, "minutes": minutes(resetTTL),
			},
			SecretParams: map[string]string{"reset_url": s.Links.resetURL(scope, token)},
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

// ResetPassword spends the token, sets the password and ends every session the old password opened.
func (s *AccountService) ResetPassword(ctx context.Context, scope HostScope, token, password string) error {
	if !validPassword(password) {
		return &InputError{Fields: map[string]string{"password": ReasonPassword}}
	}
	if token == "" {
		return ErrInvalidResetToken
	}
	// Hashing before the transaction keeps the row lock short.
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := s.now()
	var user model.User
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		reset, err := s.Accounts.ConsumeResetTx(ctx, tx, scopeTenant(scope), hashRefreshToken(token), now)
		if errors.Is(err, repository.ErrNotFound) {
			return ErrInvalidResetToken
		}
		if err != nil {
			return err
		}
		user, err = s.Users.FindByIDTx(ctx, tx, scopeTenant(scope), reset.UserID)
		if errors.Is(err, repository.ErrNotFound) {
			return ErrInvalidResetToken
		}
		if err != nil {
			return err
		}
		if !user.IsActive {
			return ErrInvalidResetToken
		}
		if err := s.Users.UpdatePasswordTx(ctx, tx, user.TenantID, user.ID, string(hash)); err != nil {
			return err
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
		if err := s.Throttles.Clear(ctx, throttleKey(loginThrottle, scopeTenantLabel(scope), *id)); err != nil {
			s.Logger.Error("clear login throttle after reset", slog.String("error", err.Error()))
		}
	}
	s.Logger.Info("password reset",
		slog.String("user_id", user.ID.String()),
		slog.String("tenant_id", tenantLabel(user.TenantID)),
	)
	return nil
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

func (s *AccountService) sendCode(ctx context.Context, tx pgx.Tx, scope HostScope, reg model.Registration, code string) error {
	tenantID := reg.TenantID
	return s.Notifications.Enqueue(ctx, tx, NewNotification{
		Scope:     Scope{TenantID: &tenantID},
		EventKey:  "registration-code:" + reg.CodeID.String(),
		Channel:   string(model.ChannelEmail),
		Recipient: reg.Email,
		Template:  TemplateRegistrationCode,
		Params: map[string]string{
			"tenant_name": tenantName(scope), "name": reg.Name, "minutes": minutes(codeTTL),
		},
		SecretParams: map[string]string{"code": code},
		// A code that reaches the inbox after it stopped working is worse than none.
		ExpiresAt: &reg.ExpiresAt,
	})
}

func (s *AccountService) codeHash(codeID uuid.UUID, code string) []byte {
	mac := hmac.New(sha256.New, s.codeKey)
	mac.Write(codeID[:])
	mac.Write([]byte(code))
	return mac.Sum(nil)
}

// inTx commits only when fn succeeds, except that fn may commit a failed guess by returning nil.
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

func codeSent(email string) CodeSent {
	return CodeSent{Email: email, ExpiresIn: codeTTL, ResendAfter: codeResendAfter}
}

// newCode is uniform over 000000–999999; codeID makes every send a distinct notification.
func newCode() (string, uuid.UUID, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%0*d", codeDigits, n.Int64()), uuid.New(), nil
}

func validCodeShape(code string) bool {
	if len(code) != codeDigits {
		return false
	}
	return strings.Trim(code, "0123456789") == ""
}

func tenantName(scope HostScope) string {
	if scope.Platform {
		return "Taruna Eduka"
	}
	return scope.Tenant.Name
}

func minutes(d time.Duration) string { return strconv.Itoa(int(d / time.Minute)) }
