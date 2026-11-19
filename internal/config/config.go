package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
}

type AppConfig struct {
	Name string
	Env  string
	Port string
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
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "streakit"),
			Env:  getEnv("APP_ENV", "development"),
			Port: getEnv("APP_PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:                  getEnv("DATABASE_HOST", "localhost"),
			Port:                  getEnv("DATABASE_PORT", "5432"),
			User:                  getEnv("DATABASE_USER", "postgres"),
			Password:              getEnv("DATABASE_PASSWORD", "postgres"),
			Name:                  getEnv("DATABASE_NAME", "go_backend"),
			SSLMode:               getEnv("DATABASE_SSLMODE", "disable"),
			MaxOpenConnections:    25,
			MaxIdleConnections:    5,
			ConnectionMaxLifetime: 5 * time.Minute,
		},
	}

	var err error

	cfg.Database.MaxOpenConnections, err = getIntEnv(
		"DATABASE_MAX_OPEN_CONNECTIONS", 25,
	)
	if err != nil {
		return nil, err
	}

	cfg.Database.MaxIdleConnections, err = getIntEnv(
		"DATABASE_MAX_IDLE_CONNECTIONS", 5,
	)
	if err != nil {
		return nil, err
	}

	cfg.Database.ConnectionMaxLifetime, err = time.ParseDuration(
		getEnv("DATABASE_CONNECTION_MAX_LIFETIME", "5m"),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid database connection lifetime: %w", err)
	}

	if cfg.App.Port == "" {
		return nil, fmt.Errorf("APP_PORT cannot be empty")
	}

	return cfg, nil
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
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

func getIntEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)

	if value == "" {
		return fallback, nil
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer for %s: %w", key, err)
	}

	return result, nil
}
