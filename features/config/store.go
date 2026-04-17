package config

// Store defines the contract for loading and saving configuration.
// Implementations handle serialization details such as encoding sensitive fields.
type Store interface {
	Load() (*Config, error)
	Save(cfg *Config) error
}
