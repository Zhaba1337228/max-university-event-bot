package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"

	"github.com/Zhaba1337228/max-university-event-bot/migrations"
)

const usage = `usage: migrate <command> [args]

Commands:
  up                       apply all pending migrations
  up-to <version>          apply migrations up to (and including) version
  up-by-one                apply the next pending migration
  down                     rollback the most recently applied migration
  down-to <version>        rollback all migrations down to (but not including) version
  redo                     rollback the most recent migration then re-apply it
  status                   list applied / pending migrations
  version                  print current schema version
  reset                    rollback all migrations (DANGEROUS)
  create <name> <type>     create a new migration file (sql|go)

Required env:
  DATABASE_URL=(postgres connection string, for local use add sslmode=disable)
`

var errUsage = errors.New("usage")

func main() {
	if err := run(); err != nil {
		if errors.Is(err, errUsage) {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		return errUsage
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		return errUsage
	}

	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		return errors.New("DATABASE_URL is required (set in .env or env)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	switch cmd {
	case "up", "up-by-one", "down", "redo", "status", "version", "reset":
		return goose.RunContext(ctx, cmd, db, ".")
	case "up-to", "down-to":
		if len(args) < 1 {
			return fmt.Errorf("%s requires <version> argument", cmd)
		}
		return goose.RunContext(ctx, cmd, db, ".", args...)
	case "create":
		goose.SetBaseFS(nil)
		if len(args) < 2 {
			return fmt.Errorf("create requires <name> <type> (e.g. add_users sql)")
		}
		return goose.RunContext(ctx, cmd, db, "migrations", args...)
	default:
		return fmt.Errorf("unknown command: %q\n\n%s", cmd, usage)
	}
}
