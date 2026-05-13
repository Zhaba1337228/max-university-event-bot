package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/google/uuid"
)

// QRPayload — содержимое QR-кода приглашения.
//
// Формат текста: "MAXUEB:<event_id>:<attendance_code>"
// Префикс «MAXUEB» помогает отсечь чужие QR-коды на странице сканирования.
type QRPayload struct {
	EventID        int64
	AttendanceCode string
}

const qrPrefix = "MAXUEB:"

// Ошибки QR.
var (
	ErrQRInvalidPrefix = errors.New("qr: wrong prefix")
	ErrQRInvalidFormat = errors.New("qr: invalid format")
)

// QR — сервис генерации/парсинга QR-кодов.
type QR interface {
	// NewAttendanceCode возвращает 32-символьный hex-код (uuid v4 без дефисов).
	// 128 бит энтропии — практически непредсказуемо.
	NewAttendanceCode() string

	// BuildQRPayload собирает payload в формате MAXUEB:<eventID>:<code>.
	BuildQRPayload(eventID int64, code string) string

	// ParseQRPayload разбирает payload, проверяет префикс и формат.
	// Возвращает ErrQRInvalidPrefix если префикс «MAXUEB:» отсутствует.
	ParseQRPayload(payload string) (*QRPayload, error)

	// GenerateQRPNG возвращает PNG-байты QR-кода с уровнем коррекции Medium
	// и размером 512x512 (хорошо читается с экрана телефона).
	GenerateQRPNG(payload string) ([]byte, error)
}

type qrService struct{}

// NewQR создаёт сервис.
func NewQR() QR { return &qrService{} }

func (qrService) NewAttendanceCode() string {
	// uuid v4 hex без дефисов — 32 символа, влезает в CHAR(32) колонки.
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func (qrService) BuildQRPayload(eventID int64, code string) string {
	return qrPrefix + strconv.FormatInt(eventID, 10) + ":" + code
}

func (qrService) ParseQRPayload(payload string) (*QRPayload, error) {
	if !strings.HasPrefix(payload, qrPrefix) {
		return nil, ErrQRInvalidPrefix
	}
	rest := strings.TrimPrefix(payload, qrPrefix)
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return nil, ErrQRInvalidFormat
	}
	eid, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || eid <= 0 {
		return nil, fmt.Errorf("%w: bad event_id", ErrQRInvalidFormat)
	}
	if len(parts[1]) != 32 {
		return nil, fmt.Errorf("%w: code must be 32 hex chars, got %d", ErrQRInvalidFormat, len(parts[1]))
	}
	return &QRPayload{EventID: eid, AttendanceCode: parts[1]}, nil
}

func (qrService) GenerateQRPNG(payload string) ([]byte, error) {
	// Medium recovery (~15%) — хороший баланс между размером и устойчивостью
	// к загрязнению экрана; 512 px достаточно для скана с метра.
	png, err := qrcode.Encode(payload, qrcode.Medium, 512)
	if err != nil {
		return nil, fmt.Errorf("generate qr: %w", err)
	}
	return png, nil
}
