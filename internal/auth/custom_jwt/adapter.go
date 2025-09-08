package custom_jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Adapter implements token issuance as an auth.Method via registry.
// It creates a signed JWT (HS256) with optional standard claims and arbitrary custom fields.
// Acquire returns the Authorization header value: "Bearer <token>".
// Note: Verification middleware for pkg/router is provided in middleware.go.

type Adapter struct {
	C Config
}

type Config struct {
	// Secret is the HMAC secret key used for HS256 signing (required)
	Secret string `mapstructure:"secret" json:"secret" yaml:"secret"`
	// TTL controls expiration when Exp is not provided. Default 5 minutes.
	TTLSeconds int64 `mapstructure:"ttl_seconds" json:"ttl_seconds" yaml:"ttl_seconds"`

	// Optional standard claims
	Subject   string   `mapstructure:"sub" json:"sub" yaml:"sub"`
	Issuer    string   `mapstructure:"iss" json:"iss" yaml:"iss"`
	Audience  []string `mapstructure:"aud" json:"aud" yaml:"aud"`
	NotBefore int64    `mapstructure:"nbf" json:"nbf" yaml:"nbf"`
	ExpiresAt int64    `mapstructure:"exp" json:"exp" yaml:"exp"`
	ID        string   `mapstructure:"jti" json:"jti" yaml:"jti"`

	// Custom is a bag for arbitrary custom fields to embed into the token (e.g., attrs map)
	Custom map[string]interface{} `mapstructure:"custom" json:"custom" yaml:"custom"`
}

// ToMap allows reuse via registry API if needed.
func (c Config) ToMap() map[string]interface{} { return map[string]interface{}{"secret": c.Secret} }

func (a Adapter) Acquire(_ interface{ Done() <-chan struct{} }) (string, error) {
	return "", errors.New("not used")
}

// Ensure Adapter satisfies interface at compile time via wrapper type in registry.go

// Issue creates a signed JWT token string from Config.
func (c Config) Issue() (string, error) {
	if len(c.Secret) == 0 {
		return "", errors.New("custom_jwt: secret required")
	}
	now := time.Now()
	exp := c.ExpiresAt
	if exp == 0 {
		ttl := c.TTLSeconds
		if ttl <= 0 {
			ttl = 300
		}
		exp = now.Unix() + ttl
	}
	claims := jwt.MapClaims{}
	if c.Subject != "" {
		claims["sub"] = c.Subject
	}
	if c.Issuer != "" {
		claims["iss"] = c.Issuer
	}
	if len(c.Audience) > 0 {
		claims["aud"] = c.Audience
	}
	if c.NotBefore > 0 {
		claims["nbf"] = c.NotBefore
	}
	if c.ID != "" {
		claims["jti"] = c.ID
	}
	claims["exp"] = exp
	for k, v := range c.Custom {
		claims[k] = v
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(c.Secret))
}
