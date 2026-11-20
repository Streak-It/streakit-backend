package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Database  DatabaseConfig
	Auth      AuthConfig
	RateLimit RateLimitConfig
	Bootstrap BootstrapConfig
}

type AppConfig struct {
	Name            string
	Env             string
	Port            string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
	// TrustedProxies is the list of proxy IPs/CIDRs whose X-Forwarded-For
	// headers are honoured. Empty means trust none (use the TCP peer address).
	TrustedProxies []string
	// CORSOrigins are exact origins allowed to call with credentials.
	// Empty disables CORS entirely (same-origin / same-site only).
	CORSOrigins []string
	// SwaggerEnabled mounts the Swagger UI and OpenAPI doc at /swagger.
	SwaggerEnabled bool
}

func (c AppConfig) IsProduction() bool { return c.Env == "production" }

// IsDeployed is true for shared environments (staging, production).
func (c AppConfig) IsDeployed() bool { return c.Env == "production" || c.Env == "staging" }

type AuthConfig struct {
	JWTSecret    string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	CookieSecure bool
	// CookieDomain empty = host-only cookies.
	CookieDomain string
	// BcryptCost <= 0 uses bcrypt.DefaultCost.
	BcryptCost int
}

type RateLimitConfig struct {
	LoginPerIP    int
	LoginPerEmail int
	LoginWindow   time.Duration
}

// BootstrapConfig seeds the first super admin when none exists.
type BootstrapConfig struct {
	Email    string
	Password string
	Name     string
}

type DatabaseConfig struct {
	Host                  string
	Port                  string
	User                  string
	Password              string
	Name                  string
	SSLMode               string
	MaxOpenConnections    int
	MaxIdleConnections    int
	ConnectionMaxLifetime time.Duration
	ConnectionMaxIdleTime time.Duration
	AutoMigrate           bool
}

func Load() (*Config, error) {
	// .env is optional; in containers env vars are injected directly.
	_ = godotenv.Load()

	var errs []error
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	env := getEnv("APP_ENV", "development")

	cfg := &Config{
		App: AppConfig{
			Name:           getEnv("APP_NAME", "streakit"),
			Env:            env,
			Port:           getEnv("APP_PORT", "8080"),
			TrustedProxies: getListEnv("APP_TRUSTED_PROXIES"),
			CORSOrigins:    getListEnv("CORS_ORIGINS"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DATABASE_HOST", "localhost"),
			Port:     getEnv("DATABASE_PORT", "5432"),
			User:     getEnv("DATABASE_USER", "postgres"),
			Password: getEnv("DATABASE_PASSWORD", "postgres"),
			Name:     getEnv("DATABASE_NAME", "streakit_api"),
			SSLMode:  getEnv("DATABASE_SSLMODE", "disable"),
		},
		Auth: AuthConfig{
			// Not trimmed by getEnv semantics beyond whitespace; secrets are opaque.
			JWTSecret:    getEnv("JWT_SECRET", ""),
			CookieDomain: getEnv("COOKIE_DOMAIN", ""),
		},
		Bootstrap: BootstrapConfig{
			Email: getEnv("SUPERADMIN_EMAIL", ""),
			// Passwords are taken verbatim (no trimming).
			Password: os.Getenv("SUPERADMIN_PASSWORD"),
			Name:     getEnv("SUPERADMIN_NAME", ""),
		},
	}

	var err error

	cfg.App.LogLevel, err = getLogLevelEnv("APP_LOG_LEVEL", slog.LevelInfo)
	collect(err)
	cfg.App.ShutdownTimeout, err = getDurationEnv("APP_SHUTDOWN_TIMEOUT", 10*time.Second)
	collect(err)
	cfg.App.SwaggerEnabled, err = getBoolEnv("SWAGGER_ENABLED", true)
	collect(err)

	cfg.Database.MaxOpenConnections, err = getIntEnv("DATABASE_MAX_OPEN_CONNECTIONS", 25)
	collect(err)
	cfg.Database.MaxIdleConnections, err = getIntEnv("DATABASE_MAX_IDLE_CONNECTIONS", 5)
	collect(err)
	cfg.Database.ConnectionMaxLifetime, err = getDurationEnv("DATABASE_CONNECTION_MAX_LIFETIME", 5*time.Minute)
	collect(err)
	cfg.Database.ConnectionMaxIdleTime, err = getDurationEnv("DATABASE_CONNECTION_MAX_IDLE_TIME", 1*time.Minute)
	collect(err)
	// Auto-migrate is convenient locally but dangerous in production;
	// default it off there.
	cfg.Database.AutoMigrate, err = getBoolEnv("DATABASE_AUTO_MIGRATE", env != "production")
	collect(err)

	deployed := env == "production" || env == "staging"
	cfg.Auth.AccessTTL, err = getDurationEnv("JWT_ACCESS_TTL", 15*time.Minute)
	collect(err)
	cfg.Auth.RefreshTTL, err = getDurationEnv("JWT_REFRESH_TTL", 168*time.Hour)
	collect(err)
	cfg.Auth.CookieSecure, err = getBoolEnv("COOKIE_SECURE", deployed)
	collect(err)
	cfg.Auth.BcryptCost, err = getIntEnv("BCRYPT_COST", 0)
	collect(err)

	cfg.RateLimit.LoginPerIP, err = getIntEnv("LOGIN_RATE_LIMIT_PER_IP", 20)
	collect(err)
	cfg.RateLimit.LoginPerEmail, err = getIntEnv("LOGIN_RATE_LIMIT_PER_EMAIL", 5)
	collect(err)
	cfg.RateLimit.LoginWindow, err = getDurationEnv("LOGIN_RATE_LIMIT_WINDOW", 15*time.Minute)
	collect(err)

	collect(cfg.validate())

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var errs []error

	switch c.App.Env {
	case "development", "test", "staging", "production":
	default:
		errs = append(errs, fmt.Errorf("APP_ENV must be one of development|test|staging|production, got %q", c.App.Env))
	}

	if port, err := strconv.Atoi(c.App.Port); err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Errorf("APP_PORT must be a valid port, got %q", c.App.Port))
	}

	if c.App.ShutdownTimeout <= 0 {
		errs = append(errs, errors.New("APP_SHUTDOWN_TIMEOUT must be positive"))
	}

	errs = append(errs, c.validateAuth()...)

	if c.RateLimit.LoginWindow <= 0 {
		errs = append(errs, errors.New("LOGIN_RATE_LIMIT_WINDOW must be positive"))
	}

	for _, origin := range c.App.CORSOrigins {
		if origin == "*" {
			errs = append(errs, errors.New("CORS_ORIGINS must list exact origins; \"*\" is not allowed with credentials"))
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || (u.Path != "" && u.Path != "/") {
			errs = append(errs, fmt.Errorf("CORS_ORIGINS entry %q must look like https://app.example.com", origin))
		}
	}

	db := c.Database
	if db.MaxOpenConnections < 1 {
		errs = append(errs, errors.New("DATABASE_MAX_OPEN_CONNECTIONS must be >= 1"))
	}
	if db.MaxIdleConnections < 0 || db.MaxIdleConnections > db.MaxOpenConnections {
		errs = append(errs, errors.New("DATABASE_MAX_IDLE_CONNECTIONS must be between 0 and DATABASE_MAX_OPEN_CONNECTIONS"))
	}

	switch db.SSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		errs = append(errs, fmt.Errorf("invalid DATABASE_SSLMODE %q", db.SSLMode))
	}

	if c.App.IsProduction() {
		if db.Password == "postgres" || db.Password == "" {
			errs = append(errs, errors.New("DATABASE_PASSWORD must be set to a non-default value in production"))
		}
		if db.SSLMode == "disable" {
			slog.Warn("DATABASE_SSLMODE=disable in production")
		}
	}

	return errors.Join(errs...)
}

// placeholderSecrets are values from examples/docs that must never reach a
// deployed environment.
var placeholderSecrets = []string{"change-me", "changeme", "replace-me", "placeholder", "your-secret", "secret", "example"}

func (c *Config) validateAuth() []error {
	var errs []error
	a := c.Auth

	if len(a.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET is required and must be at least 32 bytes"))
	} else if c.App.IsDeployed() {
		lower := strings.ToLower(a.JWTSecret)
		for _, p := range placeholderSecrets {
			if strings.Contains(lower, p) {
				errs = append(errs, fmt.Errorf("JWT_SECRET looks like a placeholder (contains %q); generate a random value", p))
				break
			}
		}
	}

	if a.AccessTTL <= 0 {
		errs = append(errs, errors.New("JWT_ACCESS_TTL must be positive"))
	}
	if a.RefreshTTL <= a.AccessTTL {
		errs = append(errs, errors.New("JWT_REFRESH_TTL must be greater than JWT_ACCESS_TTL"))
	}
	if a.BcryptCost != 0 && (a.BcryptCost < 4 || a.BcryptCost > 31) {
		errs = append(errs, errors.New("BCRYPT_COST must be between 4 and 31"))
	}
	if c.App.IsDeployed() && !a.CookieSecure {
		slog.Warn("COOKIE_SECURE=false in a deployed environment; cookies will be sent over plain HTTP")
	}
	return errs
}

func (c DatabaseConfig) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   c.Name,
	}

	query := u.Query()
	query.Set("sslmode", c.SSLMode)
	u.RawQuery = query.Encode()

	return u.String()
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getIntEnv(key string, fallback int) (int, error) {
	value := getEnv(key, "")
	if value == "" {
		return fallback, nil
	}
	result, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer for %s: %w", key, err)
	}
	return result, nil
}

func getBoolEnv(key string, fallback bool) (bool, error) {
	value := getEnv(key, "")
	if value == "" {
		return fallback, nil
	}
	result, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid boolean for %s: %w", key, err)
	}
	return result, nil
}

func getDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := getEnv(key, "")
	if value == "" {
		return fallback, nil
	}
	result, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration for %s: %w", key, err)
	}
	return result, nil
}

func getLogLevelEnv(key string, fallback slog.Level) (slog.Level, error) {
	value := getEnv(key, "")
	if value == "" {
		return fallback, nil
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return 0, fmt.Errorf("invalid log level for %s: %w", key, err)
	}
	return level, nil
}

func getListEnv(key string) []string {
	value := getEnv(key, "")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
