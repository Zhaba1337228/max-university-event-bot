// Package gigachat — HTTP-клиент для Sber GigaChat API.
//
// Документация: https://developers.sber.ru/docs/ru/gigachat/api/overview
//
// Особенности:
//   - OAuth2 client_credentials с автообновлением access_token (TTL 30 мин).
//   - Опциональный InsecureTLS для локальной разработки без cert Минцифры.
//   - Безопасное логирование: никакого AuthKey/access_token в логах.
package gigachat

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Config — параметры клиента.
type Config struct {
	AuthKey      string // base64(client_id:client_secret) из ЛК Сбера
	Scope        string // GIGACHAT_API_PERS | GIGACHAT_API_B2B | GIGACHAT_API_CORP
	Model        string // GigaChat | GigaChat-Pro | GigaChat-Max
	OAuthURL     string // https://ngw.devices.sberbank.ru:9443/api/v2/oauth
	APIURL       string // https://gigachat.devices.sberbank.ru/api/v1
	Timeout      time.Duration
	CABundleFile string // optional PEM bundle with Russian trusted CA certs
	InsecureTLS  bool   // только для dev без cert Минцифры
	MaxTokens    int
}

// Client — потокобезопасный клиент.
type Client struct {
	cfg  Config
	http *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// ErrUnavailable возвращается при любых ошибках GigaChat — сервисный слой
// деградирует в fallback (показывает обычный список / исходный текст).
var ErrUnavailable = errors.New("gigachat unavailable")

// New создаёт клиента. Не делает сетевых вызовов до первого Chat().
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Second
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.CABundleFile != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if pem, err := os.ReadFile(cfg.CABundleFile); err == nil {
			if pool.AppendCertsFromPEM(pem) {
				tr.TLSClientConfig = &tls.Config{RootCAs: pool}
			}
		}
	}
	if cfg.InsecureTLS {
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		}
		tr.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // dev-only
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout, Transport: tr},
	}
}

// ChatMessage — одно сообщение в диалоге.
type ChatMessage struct {
	Role    string `json:"role"` // system | user | assistant
	Content string `json:"content"`
}

// ChatRequest — тело запроса /chat/completions.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	TopP        float64       `json:"top_p,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatChoice — один из вариантов ответа.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatResponse — упрощённая структура ответа.
type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Chat шлёт chat/completions запрос. На любую ошибку (auth, HTTP, JSON)
// возвращает ErrUnavailable (wrapper'нутую через fmt.Errorf для отладки).
func (c *Client) Chat(ctx context.Context, msgs []ChatMessage, temperature float64) (*ChatResponse, error) {
	if c.cfg.AuthKey == "" {
		return nil, fmt.Errorf("%w: auth key not configured", ErrUnavailable)
	}

	if err := c.ensureToken(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	body := ChatRequest{
		Model:       c.cfg.Model,
		Messages:    msgs,
		Temperature: temperature,
		MaxTokens:   c.cfg.MaxTokens,
	}
	buf, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.APIURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %v", ErrUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: http: %v", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		c.invalidateToken()
		return nil, fmt.Errorf("%w: 401 unauthorized", ErrUnavailable)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%w: status %d: %s", ErrUnavailable, resp.StatusCode, string(b))
	}

	var out ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("%w: decode: %v", ErrUnavailable, err)
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("%w: empty choices", ErrUnavailable)
	}
	return &out, nil
}

// token возвращает текущий access_token (под мьютексом, безопасно).
func (c *Client) token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accessToken
}

// ensureToken получает свежий access_token если текущий протух или отсутствует.
func (c *Client) ensureToken(ctx context.Context) error {
	c.mu.Lock()
	if c.accessToken != "" && time.Until(c.expiresAt) > 60*time.Second {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	form := url.Values{}
	form.Set("scope", c.cfg.Scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.OAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build oauth req: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+c.cfg.AuthKey)
	req.Header.Set("RqUID", uuid.NewString())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("oauth http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("oauth status %d: %s", resp.StatusCode, string(b))
	}

	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("oauth decode: %w", err)
	}
	if out.AccessToken == "" {
		return errors.New("oauth: empty access_token")
	}

	c.mu.Lock()
	c.accessToken = out.AccessToken
	c.expiresAt = time.UnixMilli(out.ExpiresAt)
	if c.expiresAt.IsZero() || time.Until(c.expiresAt) <= 0 {
		// Если сервер не вернул expires_at — используем дефолтный TTL 25 минут.
		c.expiresAt = time.Now().Add(25 * time.Minute)
	}
	c.mu.Unlock()
	return nil
}

func (c *Client) invalidateToken() {
	c.mu.Lock()
	c.accessToken = ""
	c.expiresAt = time.Time{}
	c.mu.Unlock()
}
