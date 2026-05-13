package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// Purpose — назначение JWT.
//
// magic — короткоживущий токен (5 мин), который бот выдаёт по /admin_login
// и пересылает inline-кнопкой. Фронт обменивает его на session JWT.
//
// session — долгоживущий cookie (12 ч), используется на каждом запросе.
type Purpose string

const (
	PurposeMagic   Purpose = "magic"
	PurposeSession Purpose = "session"
)

// Стандартные TTL.
const (
	magicTTL   = 5 * time.Minute
	sessionTTL = 12 * time.Hour
)

// Claims — JWT-нагрузка.
type Claims struct {
	UserID  int64       `json:"uid"`
	Role    domain.Role `json:"role"`
	Purpose Purpose     `json:"purpose"`
	jwt.RegisteredClaims
}

// Доменные ошибки auth.
var (
	ErrAuthInvalidToken      = errors.New("auth: invalid token")
	ErrAuthWrongPurpose      = errors.New("auth: wrong purpose")
	ErrAuthRoleChanged       = errors.New("auth: role changed since token issued")
	ErrAuthSessionKeyMissing = errors.New("auth: session key not configured")
)

// Auth — публичный интерфейс сервиса auth.
type Auth interface {
	// IssueMagic выдаёт magic JWT (TTL 5 мин). Доступен только organizer/admin
	// (для applicant возвращает ErrAccessDenied).
	IssueMagic(ctx context.Context, maxUserID int64) (string, error)

	// VerifyMagic парсит magic JWT, проверяет signature/exp/purpose.
	// На успех возвращает Claims; роль повторно сверяется с БД на VerifySession.
	VerifyMagic(token string) (*Claims, error)

	// IssueSession выдаёт session JWT (TTL 12 ч) для указанного user_id.
	// Роль берётся из БД, чтобы старая роль из magic'а не дала больше прав.
	IssueSession(ctx context.Context, userID int64) (string, error)

	// VerifySession парсит session JWT и сверяет роль с актуальной из БД
	// (защита от revoke через смену роли).
	VerifySession(ctx context.Context, token string) (*Claims, error)

	// SessionTTL возвращает TTL session-cookie для Set-Cookie Max-Age.
	SessionTTL() time.Duration
}

type authService struct {
	pool  repo.Querier
	users repo.UserRepo
	key   []byte // HMAC HS256 ключ
}

// NewAuth создаёт сервис. Ключ должен быть ≥32 байт (проверяется в config.Validate).
// Если ключ пустой — сервис вернёт ErrAuthSessionKeyMissing при первом запросе.
func NewAuth(pool repo.Querier, users repo.UserRepo, sessionKey string) Auth {
	return &authService{
		pool:  pool,
		users: users,
		key:   []byte(sessionKey),
	}
}

func (s *authService) IssueMagic(ctx context.Context, maxUserID int64) (string, error) {
	if len(s.key) == 0 {
		return "", ErrAuthSessionKeyMissing
	}
	u, err := s.users.GetByMaxID(ctx, s.pool, maxUserID)
	if err != nil {
		return "", fmt.Errorf("lookup user: %w", err)
	}
	if u == nil || !u.CanAccessAdminPanel() {
		return "", ErrAccessDenied
	}
	return s.issue(u.ID, u.Role, PurposeMagic, magicTTL)
}

func (s *authService) VerifyMagic(token string) (*Claims, error) {
	c, err := s.parse(token)
	if err != nil {
		return nil, err
	}
	if c.Purpose != PurposeMagic {
		return nil, ErrAuthWrongPurpose
	}
	return c, nil
}

func (s *authService) IssueSession(ctx context.Context, userID int64) (string, error) {
	if len(s.key) == 0 {
		return "", ErrAuthSessionKeyMissing
	}
	u, err := s.users.GetByID(ctx, s.pool, userID)
	if err != nil {
		return "", fmt.Errorf("lookup user: %w", err)
	}
	if u == nil || !u.CanAccessAdminPanel() {
		return "", ErrAccessDenied
	}
	return s.issue(u.ID, u.Role, PurposeSession, sessionTTL)
}

func (s *authService) VerifySession(ctx context.Context, token string) (*Claims, error) {
	c, err := s.parse(token)
	if err != nil {
		return nil, err
	}
	if c.Purpose != PurposeSession {
		return nil, ErrAuthWrongPurpose
	}

	// Сверяем актуальную роль из БД (защита от revoke через role-change).
	u, err := s.users.GetByID(ctx, s.pool, c.UserID)
	if err != nil {
		return nil, fmt.Errorf("verify session: %w", err)
	}
	if u == nil || u.Role != c.Role {
		return nil, ErrAuthRoleChanged
	}
	if !u.CanAccessAdminPanel() {
		return nil, ErrAccessDenied
	}
	return c, nil
}

func (s *authService) SessionTTL() time.Duration { return sessionTTL }

// issue — внутренняя выдача JWT с заданным TTL и purpose.
func (s *authService) issue(userID int64, role domain.Role, purpose Purpose, ttl time.Duration) (string, error) {
	now := time.Now()
	c := Claims{
		UserID:  userID,
		Role:    role,
		Purpose: purpose,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := tok.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

func (s *authService) parse(token string) (*Claims, error) {
	if len(s.key) == 0 {
		return nil, ErrAuthSessionKeyMissing
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuedAt(),
		jwt.WithExpirationRequired(),
	)
	c := &Claims{}
	if _, err := parser.ParseWithClaims(token, c, func(_ *jwt.Token) (any, error) {
		return s.key, nil
	}); err != nil {
		// Любая ошибка валидации → один типизированный ErrAuthInvalidToken,
		// чтобы каллер мог использовать errors.Is.
		return nil, fmt.Errorf("%w: %v", ErrAuthInvalidToken, err)
	}
	return c, nil
}
