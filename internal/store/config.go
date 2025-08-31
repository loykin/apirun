package store

type Config struct {
	Driver       string `mapstructure:"driver"`
	TableNames   TableNames
	DriverConfig DriverConfig
}

type DriverConfig interface {
	ToMap() map[string]interface{}
}
