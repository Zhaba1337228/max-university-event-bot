// devmagic — DEV-only CLI: выдаёт magic-link для локальной отладки веб-админки
// без живого MAX_BOT_TOKEN (см. MAX_BOT_DEV_SKIP_PING в internal/app/config.go).
//
// Usage:
//
//	go run ./cmd/devmagic <user_id>
//
// Параметры:
//   - ADMIN_SESSION_KEY — HMAC-ключ (≥32 символа), такой же, как у бота;
//   - DATABASE_URL — Postgres URL для look-up пользователя;
//   - ADMIN_WEB_BASE_URL — база URL веб-админки (default: http://localhost:3000).
//
// Пользователь должен заранее существовать в БД и иметь роль organizer / staff / admin.
// Если нет — засеять через psql:
//
//	INSERT INTO users(max_user_id, role) VALUES (999999, 'organizer');
//	INSERT INTO users(max_user_id, role) VALUES (888888, 'staff');
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"

	"github.com/joho/godotenv"

	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "devmagic:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return errors.New("usage: devmagic <user_id>  (user_id — это users.id, не max_user_id)")
	}
	userID, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		return fmt.Errorf("parse user_id: %w", err)
	}

	_ = godotenv.Load()

	key := os.Getenv("ADMIN_SESSION_KEY")
	if len(key) < 32 {
		return errors.New("ADMIN_SESSION_KEY must be set and ≥32 chars")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return errors.New("DATABASE_URL must be set")
	}
	webBase := os.Getenv("ADMIN_WEB_BASE_URL")
	if webBase == "" {
		webBase = "http://localhost:3000"
	}

	ctx := context.Background()
	pool, err := repo.NewPool(ctx, dbURL, 2, 1)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()

	users := repo.NewUsers()
	auth := service.NewAuth(pool, users, key)

	// IssueMagic берёт max_user_id, а у нас на руках users.id.
	// Поэтому подтянем max_user_id из БД.
	u, err := users.GetByID(ctx, pool, userID)
	if err != nil {
		return fmt.Errorf("lookup user by id=%d: %w", userID, err)
	}
	if u == nil {
		return fmt.Errorf("user id=%d not found", userID)
	}
	if !u.CanAccessAdminPanel() {
		return fmt.Errorf("user id=%d role=%q — not organizer/staff/admin", userID, u.Role)
	}

	token, err := auth.IssueMagic(ctx, u.MaxUserID)
	if err != nil {
		return fmt.Errorf("issue magic: %w", err)
	}

	link := fmt.Sprintf("%s/auth?t=%s", webBase, url.QueryEscape(token))
	slog.Info("magic-link готов", "user_id", u.ID, "max_user_id", u.MaxUserID, "role", u.Role)
	fmt.Println(link)
	return nil
}
