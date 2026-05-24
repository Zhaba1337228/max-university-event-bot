package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	maxbot "github.com/max-messenger/max-bot-api-client-go"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/messages"
	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

func ensureAttendanceCode(ctx context.Context, regs repo.RegistrationRepo, q repo.Querier,
	qr service.QR, regID int64,
) (*domain.Registration, string, error) {
	if regs == nil || q == nil || qr == nil {
		return nil, "", fmt.Errorf("qr dependencies are not configured")
	}

	reg, err := regs.Get(ctx, q, regID)
	if err != nil {
		return nil, "", fmt.Errorf("get registration: %w", err)
	}
	if reg == nil {
		return nil, "", fmt.Errorf("registration %d not found", regID)
	}
	if reg.AttendanceCode != nil && *reg.AttendanceCode != "" {
		return reg, *reg.AttendanceCode, nil
	}

	code := qr.NewAttendanceCode()
	if err := regs.SetAttendanceCode(ctx, q, regID, code); err != nil {
		return nil, "", fmt.Errorf("set attendance code: %w", err)
	}
	reg.AttendanceCode = &code
	return reg, code, nil
}

func deliverRegistrationQRCode(ctx context.Context, api *maxclient.Client, qr service.QR,
	regs repo.RegistrationRepo, q repo.Querier, log *slog.Logger,
	chatID, regID int64, event *domain.Event,
) error {
	reg, code, err := ensureAttendanceCode(ctx, regs, q, qr, regID)
	if err != nil {
		return err
	}

	payload := qr.BuildQRPayload(reg.EventID, code)
	png, err := qr.GenerateQRPNG(payload)
	if err != nil {
		return fmt.Errorf("generate qr png: %w", err)
	}

	fname := filepath.Join(os.TempDir(), "max_qr_"+code+".png")
	if err := os.WriteFile(fname, png, 0o600); err != nil {
		return fmt.Errorf("write tmp qr: %w", err)
	}
	defer func() { _ = os.Remove(fname) }()

	photo, err := api.Raw().Uploads.UploadPhotoFromFile(ctx, fname)
	if err != nil {
		return fmt.Errorf("upload qr photo: %w", err)
	}

	msg := maxbot.NewMessage().SetChat(chatID).
		SetText(messages.QRCaption(event, code)).
		AddPhoto(photo)

	if _, err := api.Raw().Messages.SendWithResult(ctx, msg); err != nil {
		return fmt.Errorf("send qr photo: %w", err)
	}

	if log != nil {
		log.Debug("qr delivered", "reg_id", reg.ID, "event_id", reg.EventID)
	}
	return nil
}
