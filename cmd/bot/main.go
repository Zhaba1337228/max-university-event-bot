// Package main — точка входа MAX University Event Bot.
//
// Цикл жизни:
//  1. Загрузить .env + распарсить env в Config (с маскировкой секретов).
//  2. Инициализировать slog с заданным форматом/уровнем.
//  3. Собрать App (pgxpool + max client + FSM + handlers + dispatcher + long-poll).
//  4. Запустить Run, который блокируется до сигнала SIGTERM/SIGINT.
//  5. Корректно завершить ресурсы (Shutdown закрывает pgxpool).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Zhaba1337228/max-university-event-bot/internal/app"
	"github.com/Zhaba1337228/max-university-event-bot/internal/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		// На этом этапе логгер ещё мог не успеть проинициализироваться,
		// поэтому falbback на fmt.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := app.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
	})
	slog.SetDefault(log)

	log.Info("starting bot",
		"version", "0.4.0-day4",
		"go_runtime_arch", runtimeArch(),
	)
	// Лог конфига — все секреты замаскированы через Config.String().
	log.Info("config loaded", "cfg", cfg.String())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	a, err := app.New(ctx, cfg, log)
	if err != nil {
		return fmt.Errorf("init app: %w", err)
	}
	defer a.Shutdown()

	if err := a.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("run app: %w", err)
	}

	log.Info("bot stopped gracefully")
	return nil
}

// runtimeArch — без импорта runtime ради краткости; возвращает строку для
// диагностики. Если потребуется детально — добавим runtime.GOOS/GOARCH.
func runtimeArch() string {
	return os.Getenv("GOOS") + "/" + os.Getenv("GOARCH")
}
