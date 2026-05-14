package adminapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/service"
)

// claimsKey — ключ для context.Value().
type claimsKey struct{}

const sessionCookieName = "sid"

// requireSession парсит cookie sid, валидирует через service.Auth.VerifySession
// и кладёт Claims в контекст. На отказ — 401 JSON.
func requireSession(auth service.Auth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(sessionCookieName)
			if err != nil || c.Value == "" {
				writeJSON(w, http.StatusUnauthorized, errResp("no_session", "Требуется вход"))
				return
			}
			claims, err := auth.VerifySession(r.Context(), c.Value)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, errResp("invalid_session", "Сессия истекла или недействительна"))
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// claimsFromContext извлекает Claims из контекста (после requireSession).
func claimsFromContext(ctx context.Context) (*service.Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(*service.Claims)
	return c, ok
}

// requireAdmin — пускает только admin'ов (роль из JWT session claims).
// Ставится поверх requireSession.
func requireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := claimsFromContext(r.Context())
			if !ok || c.Role != domain.RoleAdmin {
				writeJSON(w, http.StatusForbidden, errResp("admin_required", "Раздел доступен только администраторам"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// securityHeaders ставит базовый набор security-заголовков.
func securityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "no-referrer")
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			h.Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=()")
			h.Set("Content-Type-Options", "nosniff")
			next.ServeHTTP(w, r)
		})
	}
}

// corsMW — строгий CORS: только webBaseURL, credentials, минимум методов.
func corsMW(webBaseURL string) func(http.Handler) http.Handler {
	allowedOrigin := strings.TrimSuffix(webBaseURL, "/")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && origin == allowedOrigin {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", allowedOrigin)
				h.Set("Access-Control-Allow-Credentials", "true")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")
				h.Set("Access-Control-Max-Age", "3600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// originGuard — для mutating запросов (POST/PATCH/DELETE) — Origin/Referer
// должны совпадать с webBaseURL. Защита от CSRF поверх SameSite=Strict cookie.
func originGuard(webBaseURL string) func(http.Handler) http.Handler {
	allowed := strings.TrimSuffix(webBaseURL, "/")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = r.Header.Get("Referer")
				// Referer может содержать путь — оставим только origin.
				if i := strings.Index(origin, "//"); i > 0 {
					if j := strings.Index(origin[i+2:], "/"); j > 0 {
						origin = origin[:i+2+j]
					}
				}
			}
			if origin == "" || origin != allowed {
				writeJSON(w, http.StatusForbidden, errResp("origin_mismatch", "Bad origin"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// slogLogger — структурированный логгер без query/body (PII).
func slogLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// recoverMW — поднимает recover() и логирует панику; клиенту 500.
func recoverMW(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic in handler",
						"panic", fmt.Sprintf("%v", rec),
						"path", r.URL.Path,
					)
					writeJSON(w, http.StatusInternalServerError,
						errResp("internal", "Внутренняя ошибка"))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder — для логирования статуса.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// writeJSON — единая запись JSON-ответа.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// errResp — структура ошибки в API.
func errResp(code, message string) map[string]string {
	return map[string]string{"error": code, "message": message}
}
