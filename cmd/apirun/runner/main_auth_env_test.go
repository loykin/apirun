package runner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loykin/apirun/cmd/apirun/commands"
	"github.com/spf13/viper"
)

// This test verifies that when using an OAuth2 (client_credentials) provider,
// the CLI acquires a Bearer token, stores the bare token under .auth[kc], and a
// migration that sets Authorization: "Bearer {{.auth.kc}}" sends the correct header.
func TestMain_AuthEnv_BearerTokenInjected(t *testing.T) {
	// Mock OAuth2 token endpoint that returns a Bearer token
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t-cc-env", "token_type": "Bearer"})
	}))
	defer tokenSrv.Close()

	// Mock API server that validates the Authorization header
	calls := 0
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer t-cc-env" {
			t.Fatalf("expected Authorization=Bearer t-cc-env, got %q", got)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer apiSrv.Close()

	tdir := t.TempDir()

	// One migration that uses Authorization: Bearer {{.auth.kc}}
	mig := fmt.Sprintf(`---
up:
  name: env-bearer
  request:
    method: GET
    url: %s/test
    headers:
      - name: Authorization
        value: "Bearer {{.auth.kc}}"
  response:
    result_code: ["200"]
`, apiSrv.URL)
	_ = writeFile(t, tdir, "001_env_bearer.yaml", mig)

	// Config with oauth2 client_credentials provider; token_url points to tokenSrv
	cfg := fmt.Sprintf(`---
auth:
  - type: oauth2
    name: kc
    config:
      grant_type: client_credentials
      grant_config:
        client_id: svc
        client_secret: sec
        token_url: %s/token
migrate_dir: %s
`, tokenSrv.URL, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)

	// Run the up command
	if err := commands.UpCmd.RunE(commands.UpCmd, nil); err != nil {
		t.Fatalf("commands.UpCmd.RunE error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 API call, got %d", calls)
	}
}
