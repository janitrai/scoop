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
	return nil
}
