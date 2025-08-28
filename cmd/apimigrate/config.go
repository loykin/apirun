package main

type AuthConfig struct {
	// Single provider format when auth is an object
	Type   string                 `mapstructure:"type"`
	Config map[string]interface{} `mapstructure:"config"`
	// Legacy: providers array inside the object (optional, alternative to single provider)
	Providers []map[string]interface{} `mapstructure:"providers"`
}

type EnvConfig struct {
	Name         string `mapstructure:"name"`
	Value        string `mapstructure:"value"`
	ValueFromEnv string `mapstructure:"valueFromEnv"`
}

type StoreConfig struct {
	SaveResponseBody bool `mapstructure:"save_response_body"`
}

type ClientConfig struct {
	// Explicit options only
	Insecure      bool   `mapstructure:"insecure"`
	MinTLSVersion string `mapstructure:"min_tls_version"`
	MaxTLSVersion string `mapstructure:"max_tls_version"`
}

type WaitConfig struct {
	URL      string `mapstructure:"url"`
	Method   string `mapstructure:"method"`
	Status   int    `mapstructure:"status"`
	Timeout  string `mapstructure:"timeout"`
	Interval string `mapstructure:"interval"`
}

type ConfigDoc struct {
	Auth       []AuthConfig `mapstructure:"auth"`
	MigrateDir string       `mapstructure:"migrate_dir"`
	Wait       WaitConfig   `mapstructure:"wait"`
	Env        []EnvConfig  `mapstructure:"env"`
	Store      StoreConfig  `mapstructure:"store"`
	Client     ClientConfig `mapstructure:"client"`
}
