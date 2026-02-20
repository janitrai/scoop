package config

import (
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Environment string `envconfig:"ENVIRONMENT" default:"local"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`

	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
	DBMinConns  int32  `envconfig:"NP_DB_MIN_CONNS" default:"1"`
	DBMaxConns  int32  `envconfig:"NP_DB_MAX_CONNS" default:"8"`

	DefaultAdminUser               string `envconfig:"DEFAULT_ADMIN_USER" default:"admin"`
	DefaultAdminPassword           string `envconfig:"DEFAULT_ADMIN_PASSWORD" default:""`
	DefaultAdminMustChangePassword bool   `envconfig:"DEFAULT_ADMIN_MUST_CHANGE_PASSWORD" default:"false"`
	SessionTTLHours                int    `envconfig:"SESSION_TTL_HOURS" default:"168"`
	SessionCookieName              string `envconfig:"SESSION_COOKIE_NAME" default:"scoop_session"`
	SessionCookieSecure            bool   `envconfig:"SESSION_COOKIE_SECURE" default:"false"`
	CORSAllowedOrigins             string `envconfig:"CORS_ALLOWED_ORIGINS" default:""`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.DBMinConns < 0 {
		return fmt.Errorf("NP_DB_MIN_CONNS must be >= 0")
	}
	if c.DBMaxConns < 1 {
		return fmt.Errorf("NP_DB_MAX_CONNS must be >= 1")
	}
	if c.DBMinConns > c.DBMaxConns {
		return fmt.Errorf("NP_DB_MIN_CONNS (%d) cannot exceed NP_DB_MAX_CONNS (%d)", c.DBMinConns, c.DBMaxConns)
	}
	if strings.TrimSpace(c.DefaultAdminUser) == "" {
		return fmt.Errorf("DEFAULT_ADMIN_USER is required")
	}
	if c.SessionTTLHours < 1 {
		return fmt.Errorf("SESSION_TTL_HOURS must be >= 1")
	}
	if strings.TrimSpace(c.SessionCookieName) == "" {
		return fmt.Errorf("SESSION_COOKIE_NAME is required")
	}
	return nil
}

func (c *Config) CORSAllowedOriginsList() []string {
	if c == nil {
		return nil
	}

	parts := strings.Split(c.CORSAllowedOrigins, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	return origins
}
