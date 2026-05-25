package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

type usersRepo struct{}

// NewUsers создаёт репозиторий users.
func NewUsers() UserRepo { return &usersRepo{} }

const userColumns = `id, max_user_id, full_name, phone, email, role, locale,
    consent_at, consent_policy_ver, created_at, updated_at`

// EnsureByMaxID находит пользователя по max_user_id; если нет — создаёт нового
// с ролью applicant. Один SQL запрос с INSERT ... ON CONFLICT.
func (r *usersRepo) EnsureByMaxID(ctx context.Context, q Querier, maxUserID int64) (*domain.User, error) {
	const stmt = `
INSERT INTO users (max_user_id, role, locale)
VALUES ($1, 'applicant', 'ru')
ON CONFLICT (max_user_id) DO UPDATE
SET updated_at = users.updated_at
RETURNING ` + userColumns

	u := &domain.User{}
	if err := scanUser(q.QueryRow(ctx, stmt, maxUserID), u); err != nil {
		return nil, fmt.Errorf("ensure user: %w", err)
	}
	return u, nil
}

func (r *usersRepo) GetByID(ctx context.Context, q Querier, id int64) (*domain.User, error) {
	const stmt = `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	u := &domain.User{}
	if err := scanUser(q.QueryRow(ctx, stmt, id), u); err != nil {
		if IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (r *usersRepo) GetByMaxID(ctx context.Context, q Querier, maxUserID int64) (*domain.User, error) {
	const stmt = `SELECT ` + userColumns + ` FROM users WHERE max_user_id = $1`
	u := &domain.User{}
	if err := scanUser(q.QueryRow(ctx, stmt, maxUserID), u); err != nil {
		if IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by max id: %w", err)
	}
	return u, nil
}

// UpdateProfile записывает full_name и распознаёт contact как phone (если в нём
// есть цифры и нет @) или email (если есть @). Если contact == nil — поля не трогаем.
func (r *usersRepo) UpdateProfile(ctx context.Context, q Querier, id int64, fullName, contact *string) error {
	phone, email := splitContact(contact)

	const stmt = `
UPDATE users
SET full_name = COALESCE($2, full_name),
    phone     = CASE WHEN $3::text IS NULL THEN phone ELSE $3 END,
    email     = CASE WHEN $4::text IS NULL THEN email ELSE $4 END,
    updated_at = NOW()
WHERE id = $1`

	_, err := q.Exec(ctx, stmt, id, fullName, phone, email)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}

func (r *usersRepo) SetRole(ctx context.Context, q Querier, id int64, role domain.Role) error {
	const stmt = `UPDATE users SET role = $2, updated_at = NOW() WHERE id = $1`
	_, err := q.Exec(ctx, stmt, id, string(role))
	if err != nil {
		return fmt.Errorf("set role: %w", err)
	}
	return nil
}

func (r *usersRepo) GrantConsent(ctx context.Context, q Querier, id int64, policyVer string) error {
	const stmt = `
UPDATE users
SET consent_at = NOW(),
    consent_policy_ver = $2,
    updated_at = NOW()
WHERE id = $1`

	_, err := q.Exec(ctx, stmt, id, policyVer)
	if err != nil {
		return fmt.Errorf("grant consent: %w", err)
	}
	return nil
}

// List возвращает страницу пользователей. Если roleFilter != "" — фильтр по role.
// query — case-insensitive подстрока по full_name / phone / email / max_user_id.
func (r *usersRepo) List(ctx context.Context, q Querier, roleFilter domain.Role, query string, limit, offset int) ([]*domain.User, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	query = strings.TrimSpace(query)
	pattern := "%" + strings.ToLower(query) + "%"

	// Динамическая фильтрация. role=$1 (либо '' = без фильтра), query=$2.
	var countStmt, listStmt string
	if roleFilter == "" && query == "" {
		countStmt = `SELECT COUNT(*) FROM users`
		listStmt = `SELECT ` + userColumns + ` FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		var total int
		if err := q.QueryRow(ctx, countStmt).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count users: %w", err)
		}
		rows, err := q.Query(ctx, listStmt, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("list users: %w", err)
		}
		defer rows.Close()
		out := make([]*domain.User, 0, limit)
		for rows.Next() {
			u := &domain.User{}
			if err := scanUser(rows, u); err != nil {
				return nil, 0, fmt.Errorf("scan user: %w", err)
			}
			out = append(out, u)
		}
		if err := rows.Err(); err != nil {
			return nil, 0, fmt.Errorf("iter users: %w", err)
		}
		return out, total, nil
	}

	// Фильтр + поиск (общая ветка).
	roleStr := string(roleFilter)
	countStmt = `
SELECT COUNT(*) FROM users
WHERE ($1 = '' OR role = $1)
  AND ($2 = ''
       OR LOWER(COALESCE(full_name, '')) LIKE $3
       OR LOWER(COALESCE(phone, '')) LIKE $3
       OR LOWER(COALESCE(email, '')) LIKE $3
       OR CAST(max_user_id AS text) LIKE $3)`
	listStmt = `
SELECT ` + userColumns + `
FROM users
WHERE ($1 = '' OR role = $1)
  AND ($2 = ''
       OR LOWER(COALESCE(full_name, '')) LIKE $3
       OR LOWER(COALESCE(phone, '')) LIKE $3
       OR LOWER(COALESCE(email, '')) LIKE $3
       OR CAST(max_user_id AS text) LIKE $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	var total int
	if err := q.QueryRow(ctx, countStmt, roleStr, query, pattern).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	rows, err := q.Query(ctx, listStmt, roleStr, query, pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	out := make([]*domain.User, 0, limit)
	for rows.Next() {
		u := &domain.User{}
		if err := scanUser(rows, u); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iter users: %w", err)
	}
	return out, total, nil
}

// ForgetMe удаляет пользователя; FK CASCADE подчистит registrations,
// action_logs (через target_user_id/actor_user_id с ON DELETE SET NULL),
// notifications, user_states.
//
// На action_logs мы намеренно SET NULL, а не CASCADE, чтобы статистика
// (агрегаты по action) пережила удаление конкретного пользователя.
func (r *usersRepo) ForgetMe(ctx context.Context, q Querier, id int64) error {
	const stmt = `DELETE FROM users WHERE id = $1`
	_, err := q.Exec(ctx, stmt, id)
	if err != nil {
		return fmt.Errorf("forget me: %w", err)
	}
	return nil
}

// scanUser — общий хелпер для Scan. Принимает row, что упрощает
// поведение «row × pgxmock».
func scanUser(row interface{ Scan(...any) error }, u *domain.User) error {
	var role string
	if err := row.Scan(
		&u.ID, &u.MaxUserID, &u.FullName, &u.Phone, &u.Email,
		&role, &u.Locale, &u.ConsentAt, &u.ConsentPolicyVer,
		&u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		return err
	}
	u.Role = domain.Role(role)
	return nil
}

// splitContact распознаёт контакт: если содержит @ - это email,
// если содержит ≥7 цифр - телефон, иначе оба nil.
//
// Внутри: возвращает *string, чтобы COALESCE в SQL отличал «не трогать»
// от «положить пустую строку».
func splitContact(contact *string) (*string, *string) {
	if contact == nil {
		return nil, nil
	}
	c := strings.TrimSpace(*contact)
	if c == "" {
		return nil, nil
	}
	if strings.Contains(c, "@") && strings.Contains(c, ".") {
		return nil, &c
	}
	digits := 0
	for _, r := range c {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	if digits >= 7 {
		return &c, nil
	}
	// Не распознали — пишем в phone как «raw». Это безопаснее, чем игнорировать
	// ввод пользователя, который может прислать что-то нестандартное.
	return &c, nil
}
