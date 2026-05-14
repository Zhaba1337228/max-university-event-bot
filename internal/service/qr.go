package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/google/uuid"
)

// QRPayload — содержимое QR-кода приглашения после разбора.
//
// На уровне сервиса payload содержит только то, что нужно для check-in:
// идентификатор события и attendance_code (32-символьный hex, uuid v4 без
// дефисов). Срок жизни (exp) проверяется на этапе ParseQRPayload и наружу
// уже не отдаётся.
type QRPayload struct {
	EventID        int64
	AttendanceCode string
}

// Префиксы.
//
//	"MAXUEB1." — новый формат: зашифрованный JSON, base64url.
//	"MAXUEB:"  — legacy формат (event_id и attendance_code в открытом виде),
//	             принимается на чтение для уже выпущенных QR.
const (
	qrPrefixEncV1 = "MAXUEB1."
	qrPrefixLegacy = "MAXUEB:"
)

// Ошибки QR.
//
// Маппинг на user-facing сообщения — в transport/adminapi/handlers.go.
// Здесь сознательно НЕ говорим пользователю, в чём именно сломался QR
// (это бы помогло злоумышленнику подбирать формат), но логи на сервере
// отражают конкретную причину через wrapped error.
var (
	ErrQRInvalidPrefix = errors.New("qr: wrong prefix")
	ErrQRInvalidFormat = errors.New("qr: invalid format")
	ErrQRExpired       = errors.New("qr: expired")
	ErrQRTampered      = errors.New("qr: tampered")
)

// QR — сервис генерации/парсинга QR-кодов.
type QR interface {
	// NewAttendanceCode возвращает 32-символьный hex-код (uuid v4 без дефисов).
	// 128 бит энтропии — практически непредсказуемо.
	NewAttendanceCode() string

	// BuildQRPayload собирает payload в актуальном формате (MAXUEB1.<base64url>).
	// Срок жизни QR — по умолчанию 30 дней, но в реальных потоках check-in
	// токен и так короткоживущий: regularly invalidated в БД.
	BuildQRPayload(eventID int64, code string) string

	// BuildQRPayloadWithTTL — то же, но с явным сроком жизни от now().
	BuildQRPayloadWithTTL(eventID int64, code string, ttl time.Duration) string

	// ParseQRPayload разбирает payload, проверяет формат и срок жизни.
	// Принимает оба префикса: MAXUEB1. (encrypted) и MAXUEB: (legacy).
	ParseQRPayload(payload string) (*QRPayload, error)

	// GenerateQRPNG возвращает PNG-байты QR-кода с уровнем коррекции Medium
	// и размером 512x512 (хорошо читается с экрана телефона).
	GenerateQRPNG(payload string) ([]byte, error)
}

// qrService — реализация QR.
//
// При наличии 32-байтного ключа (производного от secret через SHA-256)
// генерация выдаёт зашифрованный AES-GCM payload. Чтение поддерживает
// оба формата для миграции уже выпущенных QR.
type qrService struct {
	// aead инициализирован, если secret валиден. Если nil — генерация падает
	// с ошибкой, чтобы случайно не отдать legacy-формат в проде.
	aead    cipher.AEAD
	// allowLegacyEmit — разрешить BuildQRPayload отдавать MAXUEB: вместо
	// шифрованного формата (только если ключа нет). Для тестов.
	allowLegacyEmit bool
	// defaultTTL — по умолчанию для BuildQRPayload.
	defaultTTL time.Duration
}

const defaultQRTTL = 30 * 24 * time.Hour

// NewQR создаёт сервис с шифрованием. secret должен быть ≥16 символов
// (на практике передаём ADMIN_SESSION_KEY). Ключ AES-256 выводится через
// SHA-256(secret), nonce — случайные 12 байт на каждый payload.
func NewQR(secret string) (QR, error) {
	if len(secret) < 16 {
		return nil, fmt.Errorf("qr: secret must be ≥16 chars (got %d)", len(secret))
	}
	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("qr: aes init: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("qr: gcm init: %w", err)
	}
	return &qrService{aead: aead, defaultTTL: defaultQRTTL}, nil
}

// NewQRPlaintext — фабрика без шифрования, только для тестов и dev-инструментов
// (cmd/devmagic не использует QR, но если когда-нибудь понадобится — оставим).
// В обычном app.go использовать НЕ нужно: BuildQRPayload здесь будет отдавать
// legacy-формат MAXUEB:event:code, который читается, но не защищён.
func NewQRPlaintext() QR {
	return &qrService{allowLegacyEmit: true, defaultTTL: defaultQRTTL}
}

func (s *qrService) NewAttendanceCode() string {
	// uuid v4 hex без дефисов — 32 символа, влезает в CHAR(32) колонки.
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

// payloadV1 — то, что мы кладём внутрь зашифрованного блока.
//
// Короткие имена полей — чтобы итоговая строка влезала в QR с уровнем
// коррекции Medium на телефонном экране (модулей мало → удобно сканировать).
type payloadV1 struct {
	E   int64 `json:"e"`           // event_id
	C   string `json:"c"`          // attendance_code (32 hex)
	Exp int64 `json:"exp"`         // unix-секунды
}

func (s *qrService) BuildQRPayload(eventID int64, code string) string {
	return s.BuildQRPayloadWithTTL(eventID, code, s.defaultTTL)
}

func (s *qrService) BuildQRPayloadWithTTL(eventID int64, code string, ttl time.Duration) string {
	if s.aead == nil {
		if s.allowLegacyEmit {
			return qrPrefixLegacy + strconv.FormatInt(eventID, 10) + ":" + code
		}
		// На проде так нельзя — это указывает на bug в DI. Не паникуем,
		// чтобы handler'ы не дёргали процесс, но возвращаем явно-нечитаемое
		// значение, которое ParseQRPayload отклонит.
		return "MAXUEB-misconfigured"
	}
	pl := payloadV1{E: eventID, C: code, Exp: time.Now().Add(ttl).Unix()}
	plaintext, err := json.Marshal(pl)
	if err != nil {
		// json.Marshal на простой struct не падает; на всякий случай — degrade.
		return "MAXUEB-marshal-error"
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "MAXUEB-rand-error"
	}
	// dst = nonce || ciphertext+tag
	sealed := s.aead.Seal(nil, nonce, plaintext, nil)
	blob := append(nonce, sealed...)
	return qrPrefixEncV1 + base64.RawURLEncoding.EncodeToString(blob)
}

func (s *qrService) ParseQRPayload(payload string) (*QRPayload, error) {
	switch {
	case strings.HasPrefix(payload, qrPrefixEncV1):
		return s.parseEncrypted(strings.TrimPrefix(payload, qrPrefixEncV1))
	case strings.HasPrefix(payload, qrPrefixLegacy):
		return s.parseLegacy(strings.TrimPrefix(payload, qrPrefixLegacy))
	default:
		return nil, ErrQRInvalidPrefix
	}
}

func (s *qrService) parseEncrypted(body string) (*QRPayload, error) {
	if s.aead == nil {
		// Нет ключа — расшифровать новый формат нельзя.
		return nil, fmt.Errorf("%w: encrypted qr without server key", ErrQRInvalidFormat)
	}
	blob, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("%w: bad base64", ErrQRInvalidFormat)
	}
	if len(blob) < s.aead.NonceSize()+s.aead.Overhead() {
		return nil, fmt.Errorf("%w: blob too short", ErrQRInvalidFormat)
	}
	nonce := blob[:s.aead.NonceSize()]
	ciphertext := blob[s.aead.NonceSize():]
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// auth-tag не сошёлся — payload подделан или ключ другой.
		return nil, ErrQRTampered
	}
	var pl payloadV1
	if err := json.Unmarshal(plaintext, &pl); err != nil {
		return nil, fmt.Errorf("%w: bad json: %v", ErrQRInvalidFormat, err)
	}
	if pl.E <= 0 {
		return nil, fmt.Errorf("%w: bad event_id", ErrQRInvalidFormat)
	}
	if len(pl.C) != 32 {
		return nil, fmt.Errorf("%w: code must be 32 hex chars, got %d", ErrQRInvalidFormat, len(pl.C))
	}
	if pl.Exp > 0 && time.Now().Unix() > pl.Exp {
		return nil, ErrQRExpired
	}
	return &QRPayload{EventID: pl.E, AttendanceCode: pl.C}, nil
}

func (s *qrService) parseLegacy(body string) (*QRPayload, error) {
	parts := strings.SplitN(body, ":", 2)
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

func (s *qrService) GenerateQRPNG(payload string) ([]byte, error) {
	// Medium recovery (~15%) — хороший баланс между размером и устойчивостью
	// к загрязнению экрана; 512 px достаточно для скана с метра.
	png, err := qrcode.Encode(payload, qrcode.Medium, 512)
	if err != nil {
		return nil, fmt.Errorf("generate qr: %w", err)
	}
	return png, nil
}
