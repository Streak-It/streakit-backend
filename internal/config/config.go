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
	App      AppConfig
	Database DatabaseConfig
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
}

func (c AppConfig) IsProduction() bool { return c.Env == "production" }

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
		},
		Database: DatabaseConfig{
			Host:     getEnv("DATABASE_HOST", "localhost"),
			Port:     getEnv("DATABASE_PORT", "5432"),
			User:     getEnv("DATABASE_USER", "postgres"),
			Password: getEnv("DATABASE_PASSWORD", "postgres"),
			Name:     getEnv("DATABASE_NAME", "streakit_api"),
			SSLMode:  getEnv("DATABASE_SSLMODE", "disable"),
		},
	}

	var err error

	cfg.App.LogLevel, err = getLogLevelEnv("APP_LOG_LEVEL", slog.LevelInfo)
	collect(err)
	cfg.App.ShutdownTimeout, err = getDurationEnv("APP_SHUTDOWN_TIMEOUT", 10*time.Second)
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
