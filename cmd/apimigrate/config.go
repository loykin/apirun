package main

type ConfigDoc struct {
	Auth struct {
		// New single provider format
		Type   string                 `mapstructure:"type"`
		Config map[string]interface{} `mapstructure:"config"`
		// Generic providers array for extensibility (optional, alternative to single provider)
		Providers []map[string]interface{} `mapstructure:"providers"`
	} `mapstructure:"auth"`
	MigrateDir string `mapstructure:"migrate_dir"`
	Env        []struct {
		Name         string `mapstructure:"name"`
		Value        string `mapstructure:"value"`
		ValueFromEnv string `mapstructure:"valueFromEnv"`
	} `mapstructure:"env"`
}
