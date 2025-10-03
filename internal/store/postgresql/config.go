package postgresql

import (
	"fmt"

	"github.com/loykin/apirun/internal/util"
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
	dsn, hasDSN := util.TrimEmptyCheck(p.DSN)
	host, hasHost := util.TrimEmptyCheck(p.Host)
	if !hasDSN && hasHost {
		port := p.Port
		if port == 0 {
			port = defaultPort
		}
		ssl := util.TrimWithDefault(p.SSLMode, defaultSSLMode)

		// Build DSN in the common form accepted by pgx stdlib.
		fields := util.TrimSpaceFields(p.User, p.Password, p.DBName)
		user, password, dbname := fields[0], fields[1], fields[2]
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			user, password, host, port, dbname, ssl,
		)
	}
	p.dsn = dsn
	return map[string]interface{}{
		"dsn": dsn,
	}
}
