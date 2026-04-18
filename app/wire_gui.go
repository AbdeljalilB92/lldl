package app

import (
	"github.com/AbdeljalilB92/lldl/features/auth"
	"github.com/AbdeljalilB92/lldl/features/config"
)

// WireGUIConfig holds the runtime parameters for the GUI dependency graph.
type WireGUIConfig struct {
	Concurrency int
	Delay       int
	ConfigPath  string
}

// WireForGUI creates a WailsService with auth and config providers injected.
// Authenticated features (course/video/exercise/download) are not created here
// because they depend on the token that the user provides at runtime via the
// Authenticate binding method.
func WireForGUI(cfg WireGUIConfig) *WailsService {
	authProvider := auth.NewLinkedInAuth()

	// Ensure config path is never empty — same safeguard as CLI wire.go.
	configPath := cfg.ConfigPath
	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}

	configStore := config.NewJSONStore(configPath)

	svc := NewWailsService(authProvider, configStore)
	if cfg.Concurrency > 0 {
		svc.concurrency = cfg.Concurrency
	}
	if cfg.Delay > 0 {
		svc.delay = cfg.Delay
	}

	return svc
}
