package sqlite

// SQLite configuration constants
const (
	busyTimeoutMS    = 5000 // 5 seconds in milliseconds
	foreignKeysParam = "_fk=1"
)

type Config struct {
	Path string
}

func (c *Config) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"path": c.Path,
	}
}
