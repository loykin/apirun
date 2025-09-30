package postgresql

import (
	"fmt"
	"strings"
)

// PostgreSQL configuration constants
const (
	defaultPort    = 5432
	defaultSSLMode = "disable"
)

type Config struct {
	DSN      string `mapstructure:"dsn"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	dsn      string
}

func (p *Config) ToMap() map[string]interface{} {
	// Prefer explicit DSN; otherwise, build from components when host is provided.
	dsn := strings.TrimSpace(p.DSN)
	if dsn == "" && strings.TrimSpace(p.Host) != "" {
		port := p.Port
		if port == 0 {
			port = defaultPort
		}
		ssl := strings.TrimSpace(p.SSLMode)
		if ssl == "" {
			ssl = defaultSSLMode
		}
		// Build DSN in the common form accepted by pgx stdlib.
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			strings.TrimSpace(p.User), strings.TrimSpace(p.Password),
			strings.TrimSpace(p.Host), port, strings.TrimSpace(p.DBName), ssl,
		)
	}
	p.dsn = dsn
	return map[string]interface{}{
		"dsn": dsn,
	}
}
