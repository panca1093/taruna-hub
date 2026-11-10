package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrScopeRequired = errors.New("tenant scope required")
	// Both wrap ErrDuplicate; registration needs to say which identifier is already taken.
	ErrDuplicateEmail = fmt.Errorf("%w: email", ErrDuplicate)
	ErrDuplicatePhone = fmt.Errorf("%w: phone", ErrDuplicate)
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

const userColumns = `id, tenant_id, name, email, phone, password_hash, role, balance::text, created_at, is_active, email_verified_at, avatar_key`

func (r *UserRepository) FindByID(ctx context.Context, tenantID, id uuid.UUID) (model.User, error) {
	if tenantID == uuid.Nil {
		return model.User{}, ErrScopeRequired
	}
	row := r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	return scanUser(row)
}

// FindPlatformByID is the platform-scope counterpart of FindByID: the only null-tenant read by id.
func (r *UserRepository) FindPlatformByID(ctx context.Context, id uuid.UUID) (model.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE tenant_id IS NULL AND id = $1`, id)
	return scanUser(row)
}

// FindByIdentifier backs FR-AUTH-02: one credential field matching either column.
func (r *UserRepository) FindByIdentifier(ctx context.Context, tenantID uuid.UUID, identifier string) (model.User, error) {
	if tenantID == uuid.Nil {
		return model.User{}, ErrScopeRequired
	}
	row := r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE tenant_id = $1 AND (email = $2 OR phone = $2)`,
		tenantID, identifier)
	return scanUser(row)
}

// FindPlatformByIdentifier is the only entry point that reads null-tenant accounts.
func (r *UserRepository) FindPlatformByIdentifier(ctx context.Context, identifier string) (model.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE tenant_id IS NULL AND (email = $1 OR phone = $1)`,
		identifier)
	return scanUser(row)
}

func (r *UserRepository) Create(ctx context.Context, u model.User) (model.User, error) {
	return insertUser(ctx, r.db, u)
}

// CreateTx is Create inside the caller's transaction, so an account and what made it commit together.
func (r *UserRepository) CreateTx(ctx context.Context, tx pgx.Tx, u model.User) (model.User, error) {
	return insertUser(ctx, tx, u)
}

// ReplacePasswordTx swaps the hash only while it is still the one the caller saw, so a reset link
// works once and two racing submits cannot both win. Following an emailed link proves the inbox,
// so it also verifies the email and retires any outstanding code.
func (r *UserRepository) ReplacePasswordTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID, current, next string, at time.Time) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE users
		    SET password_hash = $4, email_verified_at = coalesce(email_verified_at, $5),
		        email_code_hash = NULL, email_code_expires_at = NULL, email_code_attempts = 0
		  WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2 AND password_hash = $3`,
		id, tenantID, current, next, at)
	if err != nil {
		return false, fmt.Errorf("replace password: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// UpdateProfile rewrites the two fields a person owns; a nil phone clears it. The unique index decides a
// phone another account holds, and the email-or-phone check is the caller's to avoid.
func (r *UserRepository) UpdateProfile(ctx context.Context, tenantID, id uuid.UUID, name string, phone *string) (model.User, error) {
	if tenantID == uuid.Nil {
		return model.User{}, ErrScopeRequired
	}
	row := r.db.QueryRow(ctx,
		`UPDATE users SET name = $3, phone = $4 WHERE tenant_id = $1 AND id = $2 RETURNING `+userColumns,
		tenantID, id, name, phone)
	return scanUser(row)
}

// SetAvatar stores the picked avatar, or clears it with nil; platform accounts have a null tenant.
func (r *UserRepository) SetAvatar(ctx context.Context, tenantID *uuid.UUID, id uuid.UUID, key *string) (model.User, error) {
	row := r.db.QueryRow(ctx,
		`UPDATE users SET avatar_key = $3 WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2 RETURNING `+userColumns,
		id, tenantID, key)
	return scanUser(row)
}

// SetPasswordTx is ReplacePasswordTx without the inbox proof: a signed-in change says nothing about the email.
func (r *UserRepository) SetPasswordTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID, current, next string) (bool, error) {
	tag, err := tx.Exec(ctx,
		`UPDATE users SET password_hash = $4
		  WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2 AND password_hash = $3`,
		id, tenantID, current, next)
	if err != nil {
		return false, fmt.Errorf("set password: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// FindEmailCodeForUpdateTx locks the account row, so concurrent guesses are counted one at a time.
func (r *UserRepository) FindEmailCodeForUpdateTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID) (model.EmailCode, error) {
	var code model.EmailCode
	err := tx.QueryRow(ctx,
		`SELECT email_code_hash, email_code_expires_at, email_code_attempts, email_verified_at
		   FROM users WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2 FOR UPDATE`,
		id, tenantID).Scan(&code.Hash, &code.ExpiresAt, &code.Attempts, &code.VerifiedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.EmailCode{}, ErrNotFound
	}
	if err != nil {
		return model.EmailCode{}, fmt.Errorf("read email code: %w", err)
	}
	return code, nil
}

// SetEmailCodeTx replaces the outstanding code and restores its guess budget.
func (r *UserRepository) SetEmailCodeTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID, hash []byte, expiresAt time.Time) error {
	return execOne(ctx, tx, "set email code",
		`UPDATE users SET email_code_hash = $3, email_code_expires_at = $4, email_code_attempts = 0
		  WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		id, tenantID, hash, expiresAt)
}

func (r *UserRepository) CountEmailCodeAttemptTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID) error {
	return execOne(ctx, tx, "count email code attempt",
		`UPDATE users SET email_code_attempts = email_code_attempts + 1
		  WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		id, tenantID)
}

func (r *UserRepository) MarkEmailVerifiedTx(ctx context.Context, tx pgx.Tx, tenantID *uuid.UUID, id uuid.UUID, at time.Time) error {
	return execOne(ctx, tx, "mark email verified",
		`UPDATE users SET email_verified_at = $3,
		        email_code_hash = NULL, email_code_expires_at = NULL, email_code_attempts = 0
		  WHERE id = $1 AND tenant_id IS NOT DISTINCT FROM $2`,
		id, tenantID, at)
}

func execOne(ctx context.Context, tx pgx.Tx, what, sql string, args ...any) error {
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertUser(ctx context.Context, q rowQuerier, u model.User) (model.User, error) {
	row := q.QueryRow(ctx,
		`INSERT INTO users (tenant_id, name, email, phone, password_hash, role, email_verified_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+userColumns,
		u.TenantID, u.Name, u.Email, u.Phone, u.PasswordHash, u.Role, u.EmailVerifiedAt)
	return scanUser(row)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Phone, &u.PasswordHash, &u.Role, &u.Balance, &u.CreatedAt, &u.IsActive,
		&u.EmailVerifiedAt, &u.AvatarKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if isUniqueViolation(err) {
		return model.User{}, duplicateUser(err)
	}
	if err != nil {
		return model.User{}, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

func duplicateUser(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "users_tenant_email_key", "users_platform_email_key":
			return ErrDuplicateEmail
		case "users_tenant_phone_key", "users_platform_phone_key":
			return ErrDuplicatePhone
		}
	}
	return ErrDuplicate
}

// UserFilter narrows the staff account list; Roles is never empty, so a read cannot reach a platform account.
type UserFilter struct {
	Roles  []model.Role
	Search string
	Active *bool
	Limit  int
	Offset int
}

// List pages one tenant's accounts newest first; `%` and `_` in the search match themselves.
func (r *UserRepository) List(ctx context.Context, tenantID uuid.UUID, f UserFilter) ([]model.User, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	roles := make([]string, 0, len(f.Roles))
	for _, role := range f.Roles {
		roles = append(roles, string(role))
	}
	where := []string{"tenant_id = $1", "role = ANY($2)"}
	args := []any{tenantID, roles}
	if f.Search != "" {
		args = append(args, likePattern(f.Search))
		where = append(where, fmt.Sprintf(
			`(name ILIKE $%d ESCAPE '\' OR email ILIKE $%d ESCAPE '\' OR phone ILIKE $%d ESCAPE '\')`,
			len(args), len(args), len(args)))
	}
	if f.Active != nil {
		args = append(args, *f.Active)
		where = append(where, fmt.Sprintf("is_active = $%d", len(args)))
	}
	from := " FROM users WHERE " + strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, `SELECT `+userColumns+from+
		fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	out := make([]model.User, 0, f.Limit)
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return out, total, nil
}

// LockTx reads an account of this tenant and holds its row, so two staff actions on it run one at a time.
func (r *UserRepository) LockTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (model.User, error) {
	if tenantID == uuid.Nil {
		return model.User{}, ErrScopeRequired
	}
	return scanUser(tx.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id))
}

// LockTenantAdminsTx serializes admin deactivations of one tenant until the transaction ends.
func (r *UserRepository) LockTenantAdminsTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return ErrScopeRequired
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text || ':admins', 0))`, tenantID); err != nil {
		return fmt.Errorf("lock tenant admins: %w", err)
	}
	return nil
}

func (r *UserRepository) CountActiveAdminsTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (int, error) {
	if tenantID == uuid.Nil {
		return 0, ErrScopeRequired
	}
	var n int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE tenant_id = $1 AND role = 'admin' AND is_active`, tenantID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count active admins: %w", err)
	}
	return n, nil
}

// UpdateAccountTx rewrites the three editable fields. A changed email drops the verification, since the new
// address was not proved by its owner.
func (r *UserRepository) UpdateAccountTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, name string, email, phone *string, emailChanged bool) (model.User, error) {
	if tenantID == uuid.Nil {
		return model.User{}, ErrScopeRequired
	}
	return scanUser(tx.QueryRow(ctx, `
		UPDATE users SET name = $3, email = $4, phone = $5,
		       email_verified_at = CASE WHEN $6 THEN NULL ELSE email_verified_at END,
		       email_code_hash = CASE WHEN $6 THEN NULL ELSE email_code_hash END,
		       email_code_expires_at = CASE WHEN $6 THEN NULL ELSE email_code_expires_at END,
		       email_code_attempts = CASE WHEN $6 THEN 0 ELSE email_code_attempts END
		 WHERE tenant_id = $1 AND id = $2 RETURNING `+userColumns,
		tenantID, id, name, email, phone, emailChanged))
}

// SetActiveTx flips the account flag; every order, ledger row and result keeps pointing at the row.
func (r *UserRepository) SetActiveTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, active bool) (model.User, error) {
	if tenantID == uuid.Nil {
		return model.User{}, ErrScopeRequired
	}
	return scanUser(tx.QueryRow(ctx,
		`UPDATE users SET is_active = $3 WHERE tenant_id = $1 AND id = $2 RETURNING `+userColumns, tenantID, id, active))
}

// InsertAuditTx records an action in the caller's transaction, so it exists exactly when the action does.
func (r *UserRepository) InsertAuditTx(ctx context.Context, tx pgx.Tx, e model.AuditEvent) error {
	if e.TenantID == uuid.Nil {
		return ErrScopeRequired
	}
	detail, err := json.Marshal(e.Detail)
	if err != nil || e.Detail == nil {
		detail = []byte("{}")
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_events (tenant_id, actor_id, actor_role, action, target_type, target_id, reason, detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.TenantID, e.ActorID, e.ActorRole, e.Action, e.TargetType, e.TargetID, e.Reason, detail); err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// Audit lists one tenant's events for a target, newest first.
func (r *UserRepository) Audit(ctx context.Context, tenantID, targetID uuid.UUID) ([]model.AuditEvent, error) {
	if tenantID == uuid.Nil {
		return nil, ErrScopeRequired
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, actor_id, actor_role, action, target_type, target_id, reason, detail, created_at
		  FROM audit_events WHERE tenant_id = $1 AND target_id = $2 ORDER BY created_at DESC, id DESC`, tenantID, targetID)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()
	out := []model.AuditEvent{}
	for rows.Next() {
		var e model.AuditEvent
		var detail []byte
		if err := rows.Scan(&e.ID, &e.TenantID, &e.ActorID, &e.ActorRole, &e.Action, &e.TargetType, &e.TargetID,
			&e.Reason, &detail, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		if err := json.Unmarshal(detail, &e.Detail); err != nil {
			return nil, fmt.Errorf("decode audit detail: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// StaffGrant is the earlier manual grant that used an idempotency key, if any.
type StaffGrant struct {
	EnrollmentID uuid.UUID
	UserID       uuid.UUID
	TryoutID     *uuid.UUID
	TestID       *uuid.UUID
	Attempts     *int
}

// LockGrantKeyTx serializes concurrent requests that carry the same idempotency key.
func (r *UserRepository) LockGrantKeyTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, key string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, tenantID.String()+":staff-grant:"+key); err != nil {
		return fmt.Errorf("lock grant key: %w", err)
	}
	return nil
}

func (r *UserRepository) FindStaffGrantTx(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, key string) (StaffGrant, bool, error) {
	if tenantID == uuid.Nil {
		return StaffGrant{}, false, ErrScopeRequired
	}
	var g StaffGrant
	err := tx.QueryRow(ctx, `
		SELECT e.id, e.user_id, e.tryout_id, e.kecermatan_test_id, g.attempts
		  FROM enrollment_grants g JOIN enrollments e ON e.id = g.enrollment_id AND e.tenant_id = g.tenant_id
		 WHERE g.tenant_id = $1 AND g.source_type = 'staff' AND g.source_id = $2`, tenantID, key).
		Scan(&g.EnrollmentID, &g.UserID, &g.TryoutID, &g.TestID, &g.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return StaffGrant{}, false, nil
	}
	if err != nil {
		return StaffGrant{}, false, fmt.Errorf("find staff grant: %w", err)
	}
	return g, true, nil
}

// GrantTargetExists reports whether the tryout or kecermatan test belongs to this tenant.
func (r *UserRepository) GrantTargetExists(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, kind string) (bool, error) {
	table := "tryouts"
	extra := ""
	if kind == model.AccessKecermatan {
		table = "kecermatan_tests"
	} else {
		extra = " AND status = 'published'"
	}
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE tenant_id = $1 AND id = $2`+extra+`)`, tenantID, id).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check grant target: %w", err)
	}
	return ok, nil
}

// HoldsTryoutTx reports whether the student already has an enrollment row for the tryout.
func (r *UserRepository) HoldsTryoutTx(ctx context.Context, tx pgx.Tx, tenantID, userID, tryoutID uuid.UUID) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM enrollments WHERE tenant_id = $1 AND user_id = $2 AND tryout_id = $3)`,
		tenantID, userID, tryoutID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("check held tryout: %w", err)
	}
	return ok, nil
}

// Access lists everything one student holds, tryouts and kecermatan credits alike, newest first. The
// first grant of each enrollment says how access was opened.
func (r *UserRepository) Access(ctx context.Context, tenantID, userID uuid.UUID, now time.Time, limit, offset int) ([]model.StudentAccess, int, error) {
	return r.access(ctx, tenantID, userID, nil, now, limit, offset)
}

// AccessOf reads one enrollment of the student, the shape a grant answers with.
func (r *UserRepository) AccessOf(ctx context.Context, tenantID, userID, enrollmentID uuid.UUID, now time.Time) (model.StudentAccess, error) {
	rows, _, err := r.access(ctx, tenantID, userID, &enrollmentID, now, 1, 0)
	if err != nil {
		return model.StudentAccess{}, err
	}
	if len(rows) == 0 {
		return model.StudentAccess{}, ErrNotFound
	}
	return rows[0], nil
}

func (r *UserRepository) access(ctx context.Context, tenantID, userID uuid.UUID, enrollmentID *uuid.UUID, now time.Time, limit, offset int) ([]model.StudentAccess, int, error) {
	if tenantID == uuid.Nil {
		return nil, 0, ErrScopeRequired
	}
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM enrollments WHERE tenant_id = $1 AND user_id = $2 AND ($3::uuid IS NULL OR id = $3)`,
		tenantID, userID, enrollmentID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count access: %w", err)
	}
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.tryout_id, e.kecermatan_test_id, coalesce(t.title, k.title, ''),
		       t.status, t.active_from, t.active_until, e.attempts_purchased, e.attempts_used,
		       coalesce((SELECT g.source_type FROM enrollment_grants g
		                  WHERE g.enrollment_id = e.id AND g.tenant_id = e.tenant_id
		                  ORDER BY g.created_at, g.id LIMIT 1), 'order'),
		       e.created_at
		  FROM enrollments e
		  LEFT JOIN tryouts t ON t.id = e.tryout_id AND t.tenant_id = e.tenant_id
		  LEFT JOIN kecermatan_tests k ON k.id = e.kecermatan_test_id AND k.tenant_id = e.tenant_id
		 WHERE e.tenant_id = $1 AND e.user_id = $2 AND ($5::uuid IS NULL OR e.id = $5)
		 ORDER BY e.created_at DESC, e.id DESC LIMIT $3 OFFSET $4`, tenantID, userID, limit, offset, enrollmentID)
	if err != nil {
		return nil, 0, fmt.Errorf("list access: %w", err)
	}
	defer rows.Close()
	out := make([]model.StudentAccess, 0, limit)
	for rows.Next() {
		var a model.StudentAccess
		var tryoutID, testID *uuid.UUID
		var status *string
		if err := rows.Scan(&a.EnrollmentID, &tryoutID, &testID, &a.Title, &status, &a.ActiveFrom, &a.ActiveUntil,
			&a.AttemptsPurchased, &a.AttemptsUsed, &a.SourceType, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan access: %w", err)
		}
		if tryoutID != nil {
			a.Kind, a.TestID = model.AccessTryout, *tryoutID
			a.Active = status != nil && *status == model.TryoutPublished &&
				(a.ActiveFrom == nil || !now.Before(*a.ActiveFrom)) && a.ActiveUntil != nil && now.Before(*a.ActiveUntil)
		} else if testID != nil {
			a.Kind, a.TestID = model.AccessKecermatan, *testID
			a.Active = a.AttemptsPurchased != nil && a.AttemptsUsed < *a.AttemptsPurchased
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list access: %w", err)
	}
	return out, total, nil
}
