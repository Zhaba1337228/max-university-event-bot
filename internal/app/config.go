// Package app собирает зависимости и запускает рантайм бота.
//
// config.go — единый источник правды по env-переменным. Любая публичная
// настройка должна жить здесь, никаких прямых os.Getenv в коде сервисов.
package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"

	"github.com/Zhaba1337228/max-university-event-bot/internal/pkg/secret"
)

// Config — все настройки приложения. Группируем по доменам.
type Config struct {
	Max      MaxConfig  `envPrefix:"MAX_BOT_"`
	HTTP     HTTPConfig `envPrefix:"HTTP_"`
	Admin    AdminConfig
	DB       DBConfig
	Log      LogConfig      `envPrefix:"LOG_"`
	GigaChat GigaChatConfig `envPrefix:"GIGACHAT_"`
	Business BusinessConfig
	AI       AIConfig `envPrefix:"AI_"`
	Policy   PolicyConfig
}

// MaxConfig — параметры MAX Bot API.
type MaxConfig struct {
	Token         string        `env:"TOKEN,required"`
	Mode          string        `env:"MODE" envDefault:"longpoll"`
	WebhookURL    string        `env:"WEBHOOK_URL"`
	WebhookSecret string        `env:"WEBHOOK_SECRET"`
	HTTPTimeout   time.Duration `env:"HTTP_TIMEOUT" envDefault:"30s"`
	Debug         bool          `env:"DEBUG"`

	// DevSkipPing — выключает ping MAX API на старте, long-poll и scheduler.
	// Назначение: локальная разработка веб-админки без реального MAX_BOT_TOKEN.
	// В проде ВСЕГДА false. Включается только переменной MAX_BOT_DEV_SKIP_PING=true.
	DevSkipPing bool `env:"DEV_SKIP_PING"`
}

// HTTPConfig — webhook/healthz сервер бота.
type HTTPConfig struct {
	Addr         string        `env:"ADDR"          envDefault:":8080"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT"  envDefault:"10s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"30s"`
}

// AdminConfig — REST API + Next.js фронт.
type AdminConfig struct {
	APIAddr    string `env:"ADMIN_API_ADDR"     envDefault:":8081"`
	WebBaseURL string `env:"ADMIN_WEB_BASE_URL" envDefault:"http://localhost:3000"`
	SessionKey string `env:"ADMIN_SESSION_KEY"`
}

// DBConfig — PostgreSQL.
type DBConfig struct {
	URL      string `env:"DATABASE_URL,required"`
	MaxConns int32  `env:"DB_MAX_CONNS" envDefault:"10"`
	MinConns int32  `env:"DB_MIN_CONNS" envDefault:"2"`
}

// LogConfig — формат и уровень логов.
type LogConfig struct {
	Level  string `env:"LEVEL"  envDefault:"info"`
	Format string `env:"FORMAT" envDefault:"json"`
}

// GigaChatConfig — параметры доступа к GigaChat (День 16).
// Все поля опциональные: при пустом AuthKey AI-сервисы должны деградировать
// в fallback (см. план §17.4).
type GigaChatConfig struct {
	AuthKey     string        `env:"AUTH_KEY"`
	Scope       string        `env:"SCOPE"        envDefault:"GIGACHAT_API_PERS"`
	Model       string        `env:"MODEL"        envDefault:"GigaChat"`
	OAuthURL    string        `env:"OAUTH_URL"    envDefault:"https://ngw.devices.sberbank.ru:9443/api/v2/oauth"`
	APIURL      string        `env:"API_URL"      envDefault:"https://gigachat.devices.sberbank.ru/api/v1"`
	Timeout     time.Duration `env:"TIMEOUT"      envDefault:"20s"`
	InsecureTLS bool          `env:"INSECURE_TLS"`
}

// BusinessConfig — продуктовые настройки.
type BusinessConfig struct {
	ReminderHoursCSV    string  `env:"DEFAULT_EVENT_REMINDER_HOURS" envDefault:"24,1"`
	OrganizerUserIDs    []int64 `env:"ORGANIZER_USER_IDS" envSeparator:","`
	AdminUserIDs        []int64 `env:"ADMIN_USER_IDS"     envSeparator:","`
	WaitlistEnabled     bool    `env:"WAITLIST_ENABLED"      envDefault:"true"`
	WaitlistAutoPromote bool    `env:"WAITLIST_AUTO_PROMOTE" envDefault:"true"`
	NotifyBatchSize     int     `env:"NOTIFICATION_BATCH_SIZE"     envDefault:"50"`
	NotifyRateLimitRPS  int     `env:"NOTIFICATION_RATE_LIMIT_RPS" envDefault:"20"`
}

// AIConfig — feature flags для AI-фич.
type AIConfig struct {
	RecommenderEnabled bool          `env:"EVENT_RECOMMENDER_ENABLED"     envDefault:"true"`
	RewriterEnabled    bool          `env:"NOTIFICATION_REWRITER_ENABLED" envDefault:"true"`
	SummaryEnabled     bool          `env:"ORGANIZER_SUMMARY_ENABLED"     envDefault:"true"`
	FAQEnabled         bool          `env:"FAQ_ENABLED"                   envDefault:"false"`
	RequestTimeout     time.Duration `env:"REQUEST_TIMEOUT"               envDefault:"15s"`
	MaxTokens          int           `env:"MAX_TOKENS"                    envDefault:"512"`
}

// PolicyConfig — версия документов и т.п.
type PolicyConfig struct {
	PrivacyPolicyVersion string `env:"PRIVACY_POLICY_VERSION" envDefault:"2026-01-v1"`
}

// LoadConfig читает .env (если есть) и заполняет Config из окружения.
// Валидация — внутри Validate().
func LoadConfig() (*Config, error) {
	// .env опционален: в проде переменные приходят из docker-compose / k8s.
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

// Validate проверяет логические инварианты, которые env-парсер не видит:
// корректность режима, обязательность URL/secret для webhook, наличие
// HMAC ключа для admin API.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}

	// MAX_BOT_MODE
	mode := strings.ToLower(c.Max.Mode)
	switch mode {
	case "longpoll", "webhook":
		c.Max.Mode = mode
	default:
		return fmt.Errorf("invalid MAX_BOT_MODE: %q (want longpoll|webhook)", c.Max.Mode)
	}

	// webhook требует URL + secret
	if c.Max.Mode == "webhook" {
		if c.Max.WebhookURL == "" {
			return errors.New("MAX_BOT_WEBHOOK_URL required for webhook mode")
		}
		if c.Max.WebhookSecret == "" {
			return errors.New("MAX_BOT_WEBHOOK_SECRET required for webhook mode (§19.3)")
		}
		if len(c.Max.WebhookSecret) < 16 {
			return errors.New("MAX_BOT_WEBHOOK_SECRET must be at least 16 chars")
		}
	}

	// ADMIN_SESSION_KEY обязателен, если выставлен admin API.
	// На самом раннем этапе (только бот) допустим пустой ключ — admin API не стартует.
	if c.Admin.SessionKey != "" && len(c.Admin.SessionKey) < 32 {
		return errors.New("ADMIN_SESSION_KEY must be at least 32 chars (HMAC HS256)")
	}

	// Log
	switch strings.ToLower(c.Log.Format) {
	case "json", "text":
		c.Log.Format = strings.ToLower(c.Log.Format)
	default:
		return fmt.Errorf("invalid LOG_FORMAT: %q (want json|text)", c.Log.Format)
	}

	// GigaChat — если InsecureTLS=true и в проде, выдадим warn через String().
	// Жёсткая проверка не нужна, потому что в локалке это валидно.

	if c.Business.NotifyRateLimitRPS <= 0 || c.Business.NotifyRateLimitRPS > 30 {
		return fmt.Errorf("NOTIFICATION_RATE_LIMIT_RPS must be in [1, 30], got %d",
			c.Business.NotifyRateLimitRPS)
	}

	return nil
}

// String возвращает безопасное для логов представление конфига.
// Все секреты замаскированы через secret.Mask.
//
// Используется при `log.Info("config loaded", "cfg", cfg)`.
func (c *Config) String() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf(
		"Config{Max:{Token:%s, Mode:%s, WebhookSecret:%s}, "+
			"HTTP:%s, Admin:{API:%s, Web:%s, SessionKey:%s}, "+
			"DB:{URL:%s, MaxConns:%d}, Log:%s/%s, "+
			"GigaChat:{AuthKey:%s, Scope:%s, Model:%s, InsecureTLS:%v}, "+
			"Business:{Reminders:%s, WL:%v}, AI:{Rec:%v,Rew:%v,Sum:%v}}",
		secret.Mask(c.Max.Token), c.Max.Mode, secret.Mask(c.Max.WebhookSecret),
		c.HTTP.Addr, c.Admin.APIAddr, c.Admin.WebBaseURL, secret.Mask(c.Admin.SessionKey),
		secret.Mask(c.DB.URL), c.DB.MaxConns, c.Log.Level, c.Log.Format,
		secret.Mask(c.GigaChat.AuthKey), c.GigaChat.Scope, c.GigaChat.Model, c.GigaChat.InsecureTLS,
		c.Business.ReminderHoursCSV, c.Business.WaitlistEnabled,
		c.AI.RecommenderEnabled, c.AI.RewriterEnabled, c.AI.SummaryEnabled,
	)
}
