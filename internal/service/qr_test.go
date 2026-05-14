package service

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const testSecret = "test-secret-key-please-do-not-use-in-prod-32b"

func TestNewQR_rejectsShortSecret(t *testing.T) {
	if _, err := NewQR("short"); err == nil {
		t.Fatal("expected error for short secret")
	}
}

func TestEncryptedRoundTrip(t *testing.T) {
	qr, err := NewQR(testSecret)
	if err != nil {
		t.Fatalf("NewQR: %v", err)
	}
	code := qr.NewAttendanceCode()
	payload := qr.BuildQRPayload(42, code)

	if !strings.HasPrefix(payload, "MAXUEB1.") {
		t.Fatalf("want MAXUEB1. prefix, got %q", payload)
	}
	if strings.Contains(payload, code) {
		t.Fatalf("payload should not contain plaintext attendance_code, got %q", payload)
	}
	if strings.Contains(payload, "42") {
		// Не строгая проверка (base64 может случайно содержать '42'),
		// но event_id в виде "42:" точно не должен присутствовать.
		if strings.Contains(payload, "42:") {
			t.Fatalf("payload should not contain plaintext event_id, got %q", payload)
		}
	}

	parsed, err := qr.ParseQRPayload(payload)
	if err != nil {
		t.Fatalf("ParseQRPayload: %v", err)
	}
	if parsed.EventID != 42 {
		t.Errorf("event_id mismatch: got %d, want 42", parsed.EventID)
	}
	if parsed.AttendanceCode != code {
		t.Errorf("code mismatch: got %q, want %q", parsed.AttendanceCode, code)
	}
}

func TestEncrypted_DifferentNoncesEachCall(t *testing.T) {
	qr, _ := NewQR(testSecret)
	code := qr.NewAttendanceCode()
	a := qr.BuildQRPayload(1, code)
	b := qr.BuildQRPayload(1, code)
	if a == b {
		t.Fatal("two payloads for same inputs must differ (random nonce)")
	}
}

func TestEncrypted_TamperedDetected(t *testing.T) {
	qr, _ := NewQR(testSecret)
	code := qr.NewAttendanceCode()
	payload := qr.BuildQRPayload(1, code)

	// Меняем последний символ base64 — GCM auth-tag должен расстроиться.
	tampered := payload[:len(payload)-1] + flipChar(payload[len(payload)-1])
	_, err := qr.ParseQRPayload(tampered)
	if !errors.Is(err, ErrQRTampered) && !errors.Is(err, ErrQRInvalidFormat) {
		t.Fatalf("expected ErrQRTampered or ErrQRInvalidFormat, got %v", err)
	}
}

func TestEncrypted_ExpiredRejected(t *testing.T) {
	qr, _ := NewQR(testSecret)
	code := qr.NewAttendanceCode()
	// TTL -1h в прошлое.
	payload := qr.BuildQRPayloadWithTTL(1, code, -1*time.Hour)
	_, err := qr.ParseQRPayload(payload)
	if !errors.Is(err, ErrQRExpired) {
		t.Fatalf("expected ErrQRExpired, got %v", err)
	}
}

func TestEncrypted_WrongKeyDecryptFails(t *testing.T) {
	qrA, _ := NewQR(testSecret)
	qrB, _ := NewQR("another-secret-key-also-32b-or-longer-xyz")
	code := qrA.NewAttendanceCode()
	payload := qrA.BuildQRPayload(1, code)
	_, err := qrB.ParseQRPayload(payload)
	if !errors.Is(err, ErrQRTampered) {
		t.Fatalf("expected ErrQRTampered when decrypting with wrong key, got %v", err)
	}
}

func TestLegacyParseStillWorks(t *testing.T) {
	qr, _ := NewQR(testSecret)
	// Hand-crafted legacy payload — например, уже выпущенный QR в чатах.
	code := strings.Repeat("a", 32)
	legacy := "MAXUEB:7:" + code
	parsed, err := qr.ParseQRPayload(legacy)
	if err != nil {
		t.Fatalf("legacy ParseQRPayload: %v", err)
	}
	if parsed.EventID != 7 || parsed.AttendanceCode != code {
		t.Fatalf("legacy parse mismatch: %+v", parsed)
	}
}

func TestLegacyMode_EmitsLegacyFormat(t *testing.T) {
	qr := NewQRPlaintext()
	code := strings.Repeat("b", 32)
	payload := qr.BuildQRPayload(3, code)
	if !strings.HasPrefix(payload, "MAXUEB:") {
		t.Fatalf("plaintext mode must emit MAXUEB: prefix, got %q", payload)
	}
	parsed, err := qr.ParseQRPayload(payload)
	if err != nil {
		t.Fatalf("ParseQRPayload: %v", err)
	}
	if parsed.EventID != 3 || parsed.AttendanceCode != code {
		t.Fatalf("plaintext parse mismatch: %+v", parsed)
	}
}

func TestParseWrongPrefix(t *testing.T) {
	qr, _ := NewQR(testSecret)
	cases := []string{
		"garbage",
		"",
		"https://example.com/qr/123",
		"MAXUEB2.something",
	}
	for _, p := range cases {
		_, err := qr.ParseQRPayload(p)
		if !errors.Is(err, ErrQRInvalidPrefix) {
			t.Errorf("payload %q: expected ErrQRInvalidPrefix, got %v", p, err)
		}
	}
}

func TestParseLegacy_BadEventID(t *testing.T) {
	qr, _ := NewQR(testSecret)
	cases := []string{
		"MAXUEB:abc:" + strings.Repeat("a", 32),
		"MAXUEB:0:" + strings.Repeat("a", 32),
		"MAXUEB:-1:" + strings.Repeat("a", 32),
	}
	for _, p := range cases {
		_, err := qr.ParseQRPayload(p)
		if !errors.Is(err, ErrQRInvalidFormat) {
			t.Errorf("payload %q: expected ErrQRInvalidFormat, got %v", p, err)
		}
	}
}

func TestParseLegacy_BadCodeLength(t *testing.T) {
	qr, _ := NewQR(testSecret)
	_, err := qr.ParseQRPayload("MAXUEB:1:tooshort")
	if !errors.Is(err, ErrQRInvalidFormat) {
		t.Fatalf("expected ErrQRInvalidFormat, got %v", err)
	}
}

func TestGenerateQRPNG_NotEmpty(t *testing.T) {
	qr, _ := NewQR(testSecret)
	png, err := qr.GenerateQRPNG(qr.BuildQRPayload(1, qr.NewAttendanceCode()))
	if err != nil {
		t.Fatalf("GenerateQRPNG: %v", err)
	}
	if len(png) < 100 {
		t.Fatalf("PNG too small: %d bytes", len(png))
	}
	if string(png[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("not a PNG: prefix=%x", png[:8])
	}
}

func TestAttendanceCode_32Hex(t *testing.T) {
	qr, _ := NewQR(testSecret)
	for i := 0; i < 100; i++ {
		c := qr.NewAttendanceCode()
		if len(c) != 32 {
			t.Fatalf("len=%d, want 32, value=%q", len(c), c)
		}
		for _, r := range c {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
				t.Fatalf("non-hex char %q in code %q", r, c)
				break
			}
		}
	}
}

// flipChar возвращает другой символ из алфавита base64url.
func flipChar(c byte) string {
	if c == 'A' {
		return "B"
	}
	return "A"
}
