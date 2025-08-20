package main

import "github.com/loykin/apimigrate"

type ConfigDoc struct {
	Auth struct {
		Provider apimigrate.AuthProviderConfig `mapstructure:"provider"`
	} `mapstructure:"auth"`
	MigrateDir string `mapstructure:"migrate_dir"`
	Env        []struct {
		Name         string `mapstructure:"name"`
		Value        string `mapstructure:"value"`
		ValueFromEnv string `mapstructure:"valueFromEnv"`
	} `mapstructure:"env"`
}
