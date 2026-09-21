package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

// Config holds all environment-driven application settings.
type Config struct {
	AppEnv           string `env:"APP_ENV" envDefault:"development"`
	ServerPort       int    `env:"SERVER_PORT" envDefault:"8080"`
	DBHost           string `env:"DB_HOST" envDefault:"127.0.0.1"`
	DBPort           int    `env:"DB_PORT" envDefault:"3306"`
	DBUser           string `env:"DB_USER" envDefault:"gbexam"`
	DBPassword       string `env:"DB_PASSWORD" envDefault:"gbexam123"`
	DBName           string `env:"DB_NAME" envDefault:"gbexam"`
	JWTSecret        string `env:"JWT_SECRET" envDefault:"gbexam-dev-secret-change-me"`
	JWTExpireHours   int    `env:"JWT_EXPIRE_HOURS" envDefault:"24"`
	AdminUsername    string `env:"ADMIN_USERNAME" envDefault:"admin"`
	AdminPassword    string `env:"ADMIN_PASSWORD" envDefault:"admin123"`
	CORSAllowOrigin  string `env:"CORS_ALLOW_ORIGIN" envDefault:"*"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// DSN returns the MySQL data source name used by GORM.
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}
