package integration

import (
	"strings"
)

// Features controls which runtime capabilities are wired.
type Features struct {
	PlanTool bool
	Guard    bool
	Skills   bool
}

func DefaultFeatures() Features {
	return Features{
		PlanTool: true,
		Guard:    true,
		Skills:   true,
	}
}

// InspectOptions controls request/prompt dump behavior.
type InspectOptions struct {
	Prompt          bool
	Request         bool
	DumpDir         string
	Mode            string
	TimestampFormat string
}

// Config controls initialization and wiring behavior.
type Config struct {
	// Viper key overrides applied last (highest precedence).
	Overrides map[string]any

	Features Features
	// BuiltinToolNames optionally selects which built-in tools are wired.
	// Names are case-insensitive. When empty, all built-in tools are wired.
	BuiltinToolNames []string
	Inspect          InspectOptions
}

func DefaultConfig() Config {
	return Config{
		Overrides: map[string]any{},
		Features:  DefaultFeatures(),
		Inspect:   InspectOptions{},
	}
}

func (c *Config) Set(key string, value any) {
	if c == nil {
		return
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if c.Overrides == nil {
		c.Overrides = map[string]any{}
	}
	c.Overrides[key] = value
}
