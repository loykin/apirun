package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/loykin/apimigrate/pkg/auth"
	"github.com/loykin/apimigrate/pkg/migration"
	"gopkg.in/yaml.v3"
)

// globalAuthFile represents the YAML structure of examples/global.yaml.
// It is intentionally minimal and only maps what we need to auth.Config.
// The file may contain multiple YAML documents separated by ---.
type globalAuthFile struct {
	Auth struct {
		Provider struct {
			Name         string   `yaml:"name"`
			ClientID     string   `yaml:"client_id"`
			ClientSecret string   `yaml:"client_secret"`
			RedirectURL  string   `yaml:"redirect_url"`
			Scopes       []string `yaml:"scopes"`
			TokenHeader  string   `yaml:"token_header"`
			// Keycloak
			BaseURL  string `yaml:"base_url"`
			Realm    string `yaml:"realm"`
			Username string `yaml:"username"`
			Password string `yaml:"password"`
			// PocketBase
			Identity string `yaml:"identity"`
			IsAdmin  bool   `yaml:"is_admin"`
		} `yaml:"provider"`
	} `yaml:"auth"`
}

func main() {
	var (
		dir      = flag.String("dir", "examples/migration", "directory containing migration YAML files")
		method   = flag.String("method", "POST", "HTTP method to use when executing migrations (GET, POST, etc.)")
		url      = flag.String("url", "http://localhost:8080", "Base URL to target for migrations")
		authPath = flag.String("auth", "", "optional path to a global auth yaml to pre-acquire tokens (like examples/global.yaml)")
		verbose  = flag.Bool("v", true, "verbose output")
	)
	flag.Parse()

	ctx := context.Background()

	if strings.TrimSpace(*authPath) != "" {
		if *verbose {
			log.Printf("loading auth config from %s", *authPath)
		}
		if err := loadAndAcquireAuth(ctx, *authPath, *verbose); err != nil {
			log.Printf("warning: failed to load auth config: %v", err)
		}
	}

	if *verbose {
		log.Printf("running migrations in %s => %s %s", *dir, *method, *url)
	}
	results, err := migration.RunMigrations(ctx, *dir, *method, *url)
	if err != nil {
		// Print any partial results first
		if len(results) > 0 && *verbose {
			for i, r := range results {
				log.Printf("migration[%d]: status=%d env=%v", i, r.StatusCode, r.ExtractedEnv)
			}
		}
		log.Printf("error: %v", err)
		os.Exit(1)
	}

	if *verbose {
		for i, r := range results {
			log.Printf("migration[%d]: status=%d env=%v", i, r.StatusCode, r.ExtractedEnv)
		}
	}
	fmt.Println("migrations completed successfully")
}

func loadAndAcquireAuth(ctx context.Context, path string, verbose bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	dec := yaml.NewDecoder(f)
	loaded := 0
	for {
		var doc globalAuthFile
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, os.ErrClosed) {
				break
			}
			if err.Error() == "EOF" { // yaml v3 returns io.EOF but comparing string to avoid new import
				break
			}
			return err
		}
		pc := doc.Auth.Provider
		if strings.TrimSpace(pc.Name) == "" {
			continue
		}
		cfg := auth.Config{Provider: auth.ProviderConfig{
			Name:         pc.Name,
			TokenHeader:  pc.TokenHeader,
			ClientID:     pc.ClientID,
			ClientSecret: pc.ClientSecret,
			RedirectURL:  pc.RedirectURL,
			Scopes:       pc.Scopes,
			BaseURL:      pc.BaseURL,
			Realm:        pc.Realm,
			Username:     pc.Username,
			Password:     pc.Password,
			Identity:     pc.Identity,
			IsAdmin:      pc.IsAdmin,
		}}
		h, _, err := auth.AcquireAndStore(ctx, cfg)
		if err != nil {
			if verbose {
				log.Printf("auth provider %s: acquire failed: %v", pc.Name, err)
			}
			continue
		}
		loaded++
		if verbose {
			log.Printf("auth provider %s: using header %s", pc.Name, h)
		}
	}
	if loaded == 0 {
		return errors.New("no valid auth provider entries loaded")
	}
	return nil
}
