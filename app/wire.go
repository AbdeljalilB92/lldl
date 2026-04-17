package app

import (
	"github.com/AbdeljalilB92/lldl/features/auth"
	"github.com/AbdeljalilB92/lldl/features/config"
	"github.com/AbdeljalilB92/lldl/features/ui"
)

// WireConfig holds the runtime parameters needed to construct all concrete
// feature implementations. The Wire function uses these to build the full
// dependency graph.
type WireConfig struct {
	Concurrency int
	Delay       int
	ConfigPath  string
}

// Wire creates a fully-configured App by instantiating every concrete feature
// implementation with the correct dependencies. Authentication is performed
// eagerly because the resulting AuthenticatedClient is required by most
// downstream features.
func Wire(cfg WireConfig) (*App, error) {
	authProvider := auth.NewLinkedInAuth()

	// Ensure config path is never empty — an empty path causes atomic.WriteFile
	// to fail silently and os.ReadFile to return a confusing error.
	configPath := cfg.ConfigPath
	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}

	configStore := config.NewJSONStore(configPath)
	presenter := ui.NewCLI()

	return &App{
		authProvider: authProvider,
		configStore:  configStore,
		presenter:    presenter,
		concurrency:  cfg.Concurrency,
		delay:        cfg.Delay,
	}, nil
}
