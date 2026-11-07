package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
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
	ErrUserNotFound = errors.New("user not found")
	ErrUserExists   = errors.New("user already exists")
	ErrInvalidInput = errors.New("invalid input")
)

const minPasswordLength = 8

// e164 is the stored phone shape; Indonesian local forms are rewritten into it first.
var e164 = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// NormalizeEmail returns the stored form of an address: trimmed and lowercased, a bare addr-spec only.
func NormalizeEmail(raw string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", false
	}
	return email, true
}

// NormalizePhone returns E.164: 0812…, 62812… and +62 812-… all become +62812….
func NormalizePhone(raw string) (string, bool) {
	phone := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '-', '.', '(', ')':
			return -1
		}
		return r
	}, strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(phone, "0"):
		phone = "+62" + phone[1:]
	case strings.HasPrefix(phone, "62"):
		phone = "+" + phone
	}
	if !e164.MatchString(phone) {
		return "", false
	}
	return phone, true
}

// NormalizeIdentifier reads the one login field as an email when it has an @, else as a phone.
func NormalizeIdentifier(raw string) (string, bool) {
	if strings.Contains(raw, "@") {
		return NormalizeEmail(raw)
	}
	return NormalizePhone(raw)
}

// validPassword is the shared rule: at least 8 characters, at most bcrypt's 72 bytes.
func validPassword(password string) bool {
	return utf8.RuneCountInString(password) >= minPasswordLength && len(password) <= maxPasswordBytes
}

type UserService struct {
	users *repository.UserRepository
}

func NewUserService(users *repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) ByID(ctx context.Context, tenantID, id uuid.UUID) (model.User, error) {
	u, err := s.users.FindByID(ctx, tenantID, id)
	if errors.Is(err, repository.ErrNotFound) {
		return model.User{}, ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

func (s *UserService) ByIdentifier(ctx context.Context, tenantID uuid.UUID, identifier string) (model.User, error) {
	u, err := s.users.FindByIdentifier(ctx, tenantID, identifier)
	if errors.Is(err, repository.ErrNotFound) {
		return model.User{}, ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user by identifier: %w", err)
	}
	return u, nil
}

func (s *UserService) PlatformByIdentifier(ctx context.Context, identifier string) (model.User, error) {
	u, err := s.users.FindPlatformByIdentifier(ctx, identifier)
	if errors.Is(err, repository.ErrNotFound) {
		return model.User{}, ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find platform user by identifier: %w", err)
	}
	return u, nil
}

// ProvisionAdmin creates a tenant's first admin (or a Platform Admin when tenantID is nil); bootstrap command only.
func (s *UserService) ProvisionAdmin(ctx context.Context, tenantID *uuid.UUID, name, email, password string) (model.User, error) {
	name = strings.TrimSpace(name)
	email, ok := NormalizeEmail(email)
	if !ok || name == "" || !validPassword(password) {
		return model.User{}, ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}

	role := model.RolePlatformAdmin
	if tenantID != nil {
		role = model.RoleAdmin
	}
	u, err := s.users.Create(ctx, model.User{
		TenantID: tenantID, Name: name, Email: &email, PasswordHash: string(hash), Role: role,
	})
	if errors.Is(err, repository.ErrDuplicate) {
		return model.User{}, ErrUserExists
	}
	if err != nil {
		return model.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

var (
	// ErrUserForbidden: the role may not act on this kind of account (D02: staff accounts are Admin-only).
	ErrUserForbidden = errors.New("account action not allowed for this role")
	// ErrSelfDeactivation: an account cannot lock itself out.
	ErrSelfDeactivation = errors.New("an account cannot deactivate itself")
	ErrGrantKeyConflict = errors.New("idempotency key belongs to a different grant")
	ErrGrantTarget      = errors.New("grant target not found")
)

// Field reasons for staff account and grant input.
const (
	ReasonUserRole    = "user_role"
	ReasonUserReason  = "user_reason"
	ReasonUserKey     = "user_key"
	ReasonUserTarget  = "user_target"
	ReasonUserAttempt = "user_attempts"

	maxAuditReasonLen = 500
	maxGrantAttempts  = 1000
)

// UserAdminService runs account and grant work for staff. Deactivation is the only way an account leaves
// use (D14): the row stays, so every order, ledger entry and result keeps resolving.
type UserAdminService struct {
	db       *pgxpool.Pool
	users    *repository.UserRepository
	sessions *repository.SessionRepository
	orders   *OrderService
	cost     int
	now      func() time.Time
}

type UserAdminOption func(*UserAdminService)

// WithUserAdminClock moves the time that decides whether access is active.
func WithUserAdminClock(now func() time.Time) UserAdminOption {
	return func(s *UserAdminService) { s.now = now }
}

// WithUserAdminCost lowers the bcrypt cost for tests only.
func WithUserAdminCost(cost int) UserAdminOption {
	return func(s *UserAdminService) { s.cost = cost }
}

func NewUserAdminService(db *pgxpool.Pool, users *repository.UserRepository, sessions *repository.SessionRepository, orders *OrderService, opts ...UserAdminOption) *UserAdminService {
	s := &UserAdminService{db: db, users: users, sessions: sessions, orders: orders, cost: bcrypt.DefaultCost, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type UserListInput struct {
	// Role is "", a single role, or "staff" for operators and admins together.
	Role   string
	Search string
	Active *bool
	Limit  int
	Offset int
}

type CreateUserInput struct {
	Name     string
	Email    string
	Phone    string
	Password string
	Role     string
}

type UpdateUserInput struct {
	Name  string
	Email string
	Phone string
}

type GrantInput struct {
	TryoutID         *uuid.UUID
	KecermatanTestID *uuid.UUID
	Attempts         int
	Reason           string
	IdempotencyKey   string
}

// GrantResult is the access a grant opened; Replayed is true when the key had already done the work.
type GrantResult struct {
	Access   model.StudentAccess
	Replayed bool
}

func (s *UserAdminService) List(ctx context.Context, scope HostScope, principal model.Principal, in UserListInput) ([]model.User, int, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	roles, err := listRoles(principal, in.Role)
	if err != nil {
		return nil, 0, err
	}
	limit, offset, err := checkedWalletPage(in.Limit, in.Offset)
	if err != nil {
		return nil, 0, err
	}
	search := strings.TrimSpace(in.Search)
	if utf8.RuneCountInString(search) > 100 {
		return nil, 0, &InputError{Fields: map[string]string{"q": ReasonWalletQuery}}
	}
	users, total, err := s.users.List(ctx, tenantID, repository.UserFilter{
		Roles: roles, Search: search, Active: in.Active, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return users, total, nil
}

func (s *UserAdminService) Get(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID) (model.User, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.User{}, err
	}
	user, err := s.users.FindByID(ctx, tenantID, id)
	if err != nil {
		return model.User{}, userAdminError(err)
	}
	if err := mayManage(principal, user.Role); err != nil {
		return model.User{}, err
	}
	return user, nil
}

func (s *UserAdminService) Create(ctx context.Context, scope HostScope, principal model.Principal, in CreateUserInput) (model.User, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.User{}, err
	}
	role, fields := model.Role(in.Role), map[string]string{}
	switch role {
	case model.RoleStudent, model.RoleOperator, model.RoleAdmin:
	default:
		fields["role"] = ReasonUserRole
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > maxNameRunes {
		fields["name"] = ReasonInvalid
	}
	email, ok := NormalizeEmail(in.Email)
	if !ok {
		fields["email"] = ReasonInvalid
	}
	phone, phoneFields := optionalPhone(in.Phone)
	for k, v := range phoneFields {
		fields[k] = v
	}
	if !validPassword(in.Password) {
		fields["password"] = ReasonPassword
	}
	if len(fields) > 0 {
		return model.User{}, &InputError{Fields: fields}
	}
	if err := mayManage(principal, role); err != nil {
		return model.User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), s.cost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}
	var created model.User
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		var err error
		created, err = s.users.CreateTx(ctx, tx, model.User{
			TenantID: &tenantID, Name: name, Email: &email, Phone: phone, PasswordHash: string(hash), Role: role,
		})
		if err != nil {
			return err
		}
		return s.audit(ctx, tx, tenantID, principal, model.AuditUserCreate, model.AuditTargetUser, created.ID, "",
			map[string]string{"role": string(role)})
	})
	if err != nil {
		return model.User{}, userAdminError(err)
	}
	return created, nil
}

func (s *UserAdminService) Update(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, in UpdateUserInput) (model.User, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.User{}, err
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
	phone, phoneFields := optionalPhone(in.Phone)
	for k, v := range phoneFields {
		fields[k] = v
	}
	if len(fields) > 0 {
		return model.User{}, &InputError{Fields: fields}
	}
	var updated model.User
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.users.LockTx(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if err := mayManage(principal, current.Role); err != nil {
			return err
		}
		changed := []string{}
		if current.Name != name {
			changed = append(changed, "name")
		}
		emailChanged := current.Email == nil || *current.Email != email
		if emailChanged {
			changed = append(changed, "email")
		}
		if !samePtr(current.Phone, phone) {
			changed = append(changed, "phone")
		}
		updated, err = s.users.UpdateAccountTx(ctx, tx, tenantID, id, name, &email, phone, emailChanged)
		if err != nil {
			return err
		}
		if len(changed) == 0 {
			return nil
		}
		return s.audit(ctx, tx, tenantID, principal, model.AuditUserUpdate, model.AuditTargetUser, id, "",
			map[string]string{"fields": strings.Join(changed, ",")})
	})
	if err != nil {
		return model.User{}, userAdminError(err)
	}
	return updated, nil
}

// Deactivate blocks login and ends every live refresh session in the same transaction as the flag.
func (s *UserAdminService) Deactivate(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, rawReason string) (model.User, error) {
	return s.setActive(ctx, scope, principal, id, rawReason, false)
}

func (s *UserAdminService) Reactivate(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, rawReason string) (model.User, error) {
	return s.setActive(ctx, scope, principal, id, rawReason, true)
}

func (s *UserAdminService) setActive(ctx context.Context, scope HostScope, principal model.Principal, id uuid.UUID, rawReason string, active bool) (model.User, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return model.User{}, err
	}
	reason, err := checkedAuditReason(rawReason)
	if err != nil {
		return model.User{}, err
	}
	if !active && id == principal.UserID {
		return model.User{}, ErrSelfDeactivation
	}
	var out model.User
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		current, err := s.users.LockTx(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if err := mayManage(principal, current.Role); err != nil {
			return err
		}
		out = current
		if current.IsActive == active {
			return nil
		}
		out, err = s.users.SetActiveTx(ctx, tx, tenantID, id, active)
		if err != nil {
			return err
		}
		action := model.AuditUserReactivate
		if !active {
			action = model.AuditUserDeactivate
			if err := s.sessions.RevokeAllForUserTx(ctx, tx, id, s.now()); err != nil {
				return err
			}
		}
		return s.audit(ctx, tx, tenantID, principal, action, model.AuditTargetUser, id, reason, nil)
	})
	if err != nil {
		return model.User{}, userAdminError(err)
	}
	return out, nil
}

// Access lists what one student holds right now.
func (s *UserAdminService) Access(ctx context.Context, scope HostScope, principal model.Principal, studentID uuid.UUID, limit, offset int) ([]model.StudentAccess, int, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return nil, 0, err
	}
	limit, offset, err = checkedWalletPage(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	if err := s.requireStudent(ctx, tenantID, studentID); err != nil {
		return nil, 0, err
	}
	rows, total, err := s.users.Access(ctx, tenantID, studentID, s.now(), limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list student access: %w", err)
	}
	return rows, total, nil
}

// Grant opens one tryout, or adds kecermatan credits, for a student without an order. It goes through the
// same issuance as a purchase; the idempotency key is the grant source, so a retry grants nothing twice.
func (s *UserAdminService) Grant(ctx context.Context, scope HostScope, principal model.Principal, studentID uuid.UUID, in GrantInput) (GrantResult, error) {
	tenantID, err := walletTenant(scope, principal)
	if err != nil {
		return GrantResult{}, err
	}
	fields := map[string]string{}
	reason, err := checkedAuditReason(in.Reason)
	if err != nil {
		fields["reason"] = ReasonUserReason
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if !idempotencyKey.MatchString(key) {
		fields["idempotency_key"] = ReasonUserKey
	}
	switch {
	case (in.TryoutID == nil) == (in.KecermatanTestID == nil):
		fields["tryout_id"] = ReasonUserTarget
	case in.KecermatanTestID != nil && (in.Attempts < 1 || in.Attempts > maxGrantAttempts):
		fields["attempts"] = ReasonUserAttempt
	case in.TryoutID != nil && in.Attempts != 0:
		fields["attempts"] = ReasonUserAttempt
	}
	if len(fields) > 0 {
		return GrantResult{}, &InputError{Fields: fields}
	}

	kind, targetID := model.AccessTryout, uuid.Nil
	if in.KecermatanTestID != nil {
		kind, targetID = model.AccessKecermatan, *in.KecermatanTestID
	} else {
		targetID = *in.TryoutID
	}
	var enrollmentID uuid.UUID
	var replayed bool
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.users.LockGrantKeyTx(ctx, tx, tenantID, key); err != nil {
			return err
		}
		if _, err := s.orders.repo.LockStudentTx(ctx, tx, tenantID, studentID); err != nil {
			return err
		}
		prior, found, err := s.users.FindStaffGrantTx(ctx, tx, tenantID, key)
		if err != nil {
			return err
		}
		if found {
			if !sameGrant(prior, studentID, in) {
				return ErrGrantKeyConflict
			}
			enrollmentID, replayed = prior.EnrollmentID, true
			return nil
		}
		exists, err := s.users.GrantTargetExists(ctx, tx, tenantID, targetID, kind)
		if err != nil {
			return err
		}
		if !exists {
			return ErrGrantTarget
		}
		if kind == model.AccessTryout {
			held, err := s.users.HoldsTryoutTx(ctx, tx, tenantID, studentID, targetID)
			if err != nil {
				return err
			}
			if held {
				return ErrAlreadyOwned
			}
		}
		actor := principal.UserID
		_, err = s.orders.IssueEnrollmentsTx(ctx, tx, IssueInput{
			TenantID: tenantID, UserID: studentID, TryoutIDs: tryoutIDs(in.TryoutID),
			SourceType: model.GrantSourceStaff, SourceID: key, ActorKind: model.ActorStaff, ActorID: &actor,
			Reason: reason, KecermatanTestID: in.KecermatanTestID, Attempts: in.Attempts,
		})
		if err != nil {
			return err
		}
		granted, found, err := s.users.FindStaffGrantTx(ctx, tx, tenantID, key)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("manual grant %s left no grant row", key)
		}
		enrollmentID = granted.EnrollmentID
		detail := map[string]string{"kind": kind, "test_id": targetID.String(), "student_id": studentID.String()}
		if in.KecermatanTestID != nil {
			detail["attempts"] = fmt.Sprint(in.Attempts)
		}
		return s.audit(ctx, tx, tenantID, principal, model.AuditGrantManual, model.AuditTargetEnrollment, enrollmentID, reason, detail)
	})
	if err != nil {
		return GrantResult{}, userAdminError(err)
	}
	access, err := s.users.AccessOf(ctx, tenantID, studentID, enrollmentID, s.now())
	if err != nil {
		return GrantResult{}, fmt.Errorf("read granted access: %w", err)
	}
	return GrantResult{Access: access, Replayed: replayed}, nil
}

func (s *UserAdminService) requireStudent(ctx context.Context, tenantID, id uuid.UUID) error {
	user, err := s.users.FindByID(ctx, tenantID, id)
	if err != nil {
		return userAdminError(err)
	}
	if user.Role != model.RoleStudent {
		return ErrUserNotFound
	}
	return nil
}

func (s *UserAdminService) audit(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, principal model.Principal, action, targetType string, targetID uuid.UUID, reason string, detail map[string]string) error {
	return s.users.InsertAuditTx(ctx, tx, model.AuditEvent{
		TenantID: tenantID, ActorID: principal.UserID, ActorRole: principal.Role, Action: action,
		TargetType: targetType, TargetID: targetID, Reason: reason, Detail: detail,
	})
}

func (s *UserAdminService) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
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

// mayManage is D02: an Operator runs student accounts; Operator and Admin accounts are Admin-only.
func mayManage(principal model.Principal, target model.Role) error {
	switch {
	case principal.Role == model.RoleAdmin && target != model.RolePlatformAdmin:
		return nil
	case principal.Role == model.RoleOperator && target == model.RoleStudent:
		return nil
	}
	return ErrUserForbidden
}

func listRoles(principal model.Principal, role string) ([]model.Role, error) {
	switch role {
	case "":
		if principal.Role == model.RoleOperator {
			return []model.Role{model.RoleStudent}, nil
		}
		return []model.Role{model.RoleStudent, model.RoleOperator, model.RoleAdmin}, nil
	case "staff":
		if principal.Role != model.RoleAdmin {
			return nil, ErrUserForbidden
		}
		return []model.Role{model.RoleOperator, model.RoleAdmin}, nil
	case string(model.RoleStudent), string(model.RoleOperator), string(model.RoleAdmin):
		if err := mayManage(principal, model.Role(role)); err != nil {
			return nil, err
		}
		return []model.Role{model.Role(role)}, nil
	}
	return nil, &InputError{Fields: map[string]string{"role": ReasonUserRole}}
}

func optionalPhone(raw string) (*string, map[string]string) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	phone, ok := NormalizePhone(raw)
	if !ok {
		return nil, map[string]string{"phone": ReasonInvalid}
	}
	return &phone, nil
}

func checkedAuditReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	if reason == "" || utf8.RuneCountInString(reason) > maxAuditReasonLen {
		return "", &InputError{Fields: map[string]string{"reason": ReasonUserReason}}
	}
	return reason, nil
}

func samePtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func tryoutIDs(id *uuid.UUID) []uuid.UUID {
	if id == nil {
		return nil
	}
	return []uuid.UUID{*id}
}

func sameGrant(prior repository.StaffGrant, studentID uuid.UUID, in GrantInput) bool {
	if prior.UserID != studentID {
		return false
	}
	if in.TryoutID != nil {
		return prior.TryoutID != nil && *prior.TryoutID == *in.TryoutID
	}
	return prior.TestID != nil && *prior.TestID == *in.KecermatanTestID && prior.Attempts != nil && *prior.Attempts == in.Attempts
}

func userAdminError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrUserNotFound
	case errors.Is(err, repository.ErrDuplicateEmail):
		return ErrEmailTaken
	case errors.Is(err, repository.ErrDuplicatePhone):
		return ErrPhoneTaken
	}
	return err
}
