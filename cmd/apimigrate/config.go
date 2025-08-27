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

type ConfigDoc struct {
	Auth       []AuthConfig `mapstructure:"auth"`
	MigrateDir string       `mapstructure:"migrate_dir"`
	Env        []EnvConfig  `mapstructure:"env"`
}
