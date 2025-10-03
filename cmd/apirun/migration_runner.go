package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/internal/common"
	ienv "github.com/loykin/apirun/pkg/env"
	"github.com/spf13/viper"
)

// MigrationConfig holds all configuration needed for running migrations
type MigrationConfig struct {
	ConfigPath       string
	Dir              string
	BaseEnv          *ienv.Env
	SaveResponseBody bool
	ClientTLS        *tls.Config
	Logger           *common.Logger
}

// MigrationRunner handles the execution of migrations
type MigrationRunner struct {
	config *MigrationConfig
	ctx    context.Context
}

// NewMigrationRunner creates a new migration runner
func NewMigrationRunner(ctx context.Context) *MigrationRunner {
	return &MigrationRunner{
		ctx: ctx,
		config: &MigrationConfig{
			BaseEnv:          ienv.New(),
			SaveResponseBody: false,
		},
	}
}

// InitializeFromViper initializes the runner from Viper configuration
func (r *MigrationRunner) InitializeFromViper() error {
	v := viper.GetViper()
	r.config.ConfigPath = v.GetString("config")

	// Initialize basic logger
	logger := common.NewLogger(common.LogLevelInfo)
	common.SetDefaultLogger(logger)
	r.config.Logger = logger

	logger.Info("starting apirun", "config_path", r.config.ConfigPath)

	return nil
}

// LoadConfiguration loads and processes the configuration file
func (r *MigrationRunner) LoadConfiguration() error {
	configPath := strings.TrimSpace(r.config.ConfigPath)
	if configPath == "" {
		// Use default directory if no config
		r.config.Dir = "./migration"
		r.config.Logger.Debug("using default configuration (no config file specified)")
		return nil
	}

	r.config.Logger.Debug("loading configuration file", "path", configPath)

	var doc ConfigDoc
	if err := doc.Load(configPath); err != nil {
		return fmt.Errorf("failed to load configuration file '%s': %w\nPlease check if the file exists and has valid YAML syntax", configPath, err)
	}

	// Setup logging from config
	if err := doc.SetupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging from config: %w", err)
	}

	// Update logger after potential reconfiguration
	r.config.Logger = common.GetLogger().WithComponent("main")

	return r.processConfigDoc(&doc)
}

// processConfigDoc processes the loaded configuration document
func (r *MigrationRunner) processConfigDoc(doc *ConfigDoc) error {
	mDir := strings.TrimSpace(doc.MigrateDir)
	r.config.Logger.Debug("processing configuration", "migrate_dir", mDir)

	// Get environment from config
	envFromCfg, err := doc.GetEnv()
	if err != nil {
		r.config.Logger.Error("failed to get environment from config", "error", err)
		return err
	}

	// Perform wait check
	if err := doWait(r.ctx, envFromCfg, doc.Wait, doc.Client); err != nil {
		r.config.Logger.Error("wait check failed", "error", err)
		return fmt.Errorf("service dependency check failed: %w\nEnsure required services are running and accessible", err)
	}

	// Decode authentication
	if err := doc.DecodeAuth(r.ctx, envFromCfg); err != nil {
		r.config.Logger.Error("failed to decode authentication", "error", err)
		return fmt.Errorf("authentication configuration error: %w\nPlease check your auth settings in the config file", err)
	}

	// Set migration directory
	if mDir != "" {
		r.config.Dir = mDir
	}

	// Set environment and response body saving
	r.config.BaseEnv = envFromCfg
	r.config.SaveResponseBody = doc.Store.SaveResponseBody

	// Build TLS configuration
	r.config.ClientTLS = r.buildTLSConfig(doc.Client)

	return nil
}

// buildTLSConfig creates TLS configuration from client settings
func (r *MigrationRunner) buildTLSConfig(clientCfg ClientConfig) *tls.Config {
	minV := uint16(0)
	maxV := uint16(0)

	switch strings.TrimSpace(strings.ToLower(clientCfg.MinTLSVersion)) {
	case "1.0", "10", "tls1.0", "tls10":
		minV = tls.VersionTLS10
	case "1.1", "11", "tls1.1", "tls11":
		minV = tls.VersionTLS11
	case "1.2", "12", "tls1.2", "tls12":
		minV = tls.VersionTLS12
	case "1.3", "13", "tls1.3", "tls13":
		minV = tls.VersionTLS13
	}

	switch strings.TrimSpace(strings.ToLower(clientCfg.MaxTLSVersion)) {
	case "1.0", "10", "tls1.0", "tls10":
		maxV = tls.VersionTLS10
	case "1.1", "11", "tls1.1", "tls11":
		maxV = tls.VersionTLS11
	case "1.2", "12", "tls1.2", "tls12":
		maxV = tls.VersionTLS12
	case "1.3", "13", "tls1.3", "tls13":
		maxV = tls.VersionTLS13
	}

	cfg := &tls.Config{MinVersion: minV, MaxVersion: maxV}
	if clientCfg.Insecure {
		cfg.InsecureSkipVerify = true
	}

	r.config.Logger.Debug("TLS configuration applied",
		"insecure", clientCfg.Insecure,
		"min_version", minV,
		"max_version", maxV)

	return cfg
}

// SetDefaultDirectoryIfEmpty sets default migration directory if not configured
func (r *MigrationRunner) SetDefaultDirectoryIfEmpty() {
	if strings.TrimSpace(r.config.Dir) == "" {
		r.config.Dir = "./migration"
	}
}

// ExecuteMigrations runs the actual migrations
func (r *MigrationRunner) ExecuteMigrations() error {
	r.config.Logger.Debug("running migrations", "dir", r.config.Dir, "versioned", true)

	// Create migrator
	m := apirun.Migrator{
		Env:              r.config.BaseEnv,
		Dir:              r.config.Dir,
		SaveResponseBody: r.config.SaveResponseBody,
		TLSConfig:        r.config.ClientTLS,
	}

	// Execute migrations
	vres, err := m.MigrateUp(r.ctx, 0)
	if err != nil {
		if len(vres) > 0 {
			for _, vr := range vres {
				if vr != nil && vr.Result != nil {
					r.config.Logger.Debug("migration result",
						"version", vr.Version,
						"status_code", vr.Result.StatusCode,
						"env", vr.Result.ExtractedEnv)
				}
			}
		}
		return fmt.Errorf("migration execution failed: %w\nSome migrations may have been applied. Use 'apirun status' to check current state", err)
	}

	// Log successful results
	for _, vr := range vres {
		if vr != nil && vr.Result != nil {
			r.config.Logger.Debug("migration result",
				"version", vr.Version,
				"status_code", vr.Result.StatusCode,
				"env", vr.Result.ExtractedEnv)
		}
	}

	r.config.Logger.Info("migrations completed successfully")
	return nil
}

// Run executes the complete migration process
func (r *MigrationRunner) Run() error {
	if err := r.InitializeFromViper(); err != nil {
		return err
	}

	if err := r.LoadConfiguration(); err != nil {
		return err
	}

	r.SetDefaultDirectoryIfEmpty()

	return r.ExecuteMigrations()
}
