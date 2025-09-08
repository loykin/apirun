package custom_jwt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/loykin/apimigrate/pkg/router"
)

// VerifyConfig configures JWT verification middleware.
// Secret: HS256 secret key (required)
// RequireJTI enforces presence of jti; OneTimeUse store is not implemented here to keep minimal footprint.
// AllowedIssuer/Audience optional checks; ClockSkew tolerance.
// ClaimsKey is the context key under which verified claims are stored.

type VerifyConfig struct {
	Secret          []byte
	RequireJTI      bool
	AllowedIssuer   string
	AllowedAudience string
	ClockSkew       time.Duration
}

// GetClaimsFromContext retrieves jwt.MapClaims from request context if present.
func GetClaimsFromContext(r *http.Request) jwt.MapClaims {
	val := r.Context().Value(claimsCtxKey{})
	if c, ok := val.(jwt.MapClaims); ok {
		return c
	}
	return nil
}

type claimsCtxKey struct{}

// NewJWTMiddleware returns a router.Middleware that enforces Bearer JWT using HS256.
func NewJWTMiddleware(cfg VerifyConfig) router.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(cfg.Secret) == 0 {
				http.Error(w, "jwt secret not configured", http.StatusInternalServerError)
				return
			}
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
				return
			}
			tokStr := strings.TrimSpace(auth[len("Bearer "):])
			tok, err := jwt.Parse(tokStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return cfg.Secret, nil
			})
			if err != nil || !tok.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			claims, ok := tok.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "invalid token claims", http.StatusUnauthorized)
				return
			}
			if err := validateClaimsBasic(claims, cfg); err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsCtxKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func validateClaimsBasic(c jwt.MapClaims, cfg VerifyConfig) error {
	// time validations: exp and nbf if present
	now := time.Now()
	if v, ok := c["exp"]; ok {
		if tt, err := toTime(v); err == nil {
			if now.After(tt.Add(cfg.ClockSkew)) {
				return errors.New("token expired")
			}
		}
	}
	if v, ok := c["nbf"]; ok {
		if tt, err := toTime(v); err == nil {
			if now.Add(cfg.ClockSkew).Before(tt) {
				return errors.New("token not yet valid")
			}
		}
	}
	if cfg.RequireJTI {
		if _, ok := c["jti"]; !ok {
			return errors.New("token missing jti")
		}
	}
	if cfg.AllowedIssuer != "" {
		if iss, _ := c["iss"].(string); iss != cfg.AllowedIssuer {
			return errors.New("invalid iss")
		}
	}
	if cfg.AllowedAudience != "" {
		switch v := c["aud"].(type) {
		case string:
			if v != cfg.AllowedAudience {
				return errors.New("invalid aud")
			}
		case []interface{}:
			ok := false
			for _, it := range v {
				if s, _ := it.(string); s == cfg.AllowedAudience {
					ok = true
					break
				}
			}
			if !ok {
				return errors.New("invalid aud")
			}
		}
	}
	return nil
}

func toTime(v interface{}) (time.Time, error) {
	switch t := v.(type) {
	case float64:
		return time.Unix(int64(t), 0), nil
	case int64:
		return time.Unix(t, 0), nil
	case string:
		if n, err := time.Parse(time.RFC3339, t); err == nil {
			return n, nil
		}
		return time.Unix(0, 0), fmt.Errorf("unsupported time string")
	default:
		return time.Unix(0, 0), fmt.Errorf("unsupported time type: %T", v)
	}
}
