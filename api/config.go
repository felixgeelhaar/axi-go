package api

import (
	"os"
	"strconv"
	"time"

	"github.com/felixgeelhaar/axi-go/domain"
)

// Config holds all server configuration.
type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	DefaultBudget domain.ExecutionBudget
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Addr:         ":8080",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		DefaultBudget: domain.ExecutionBudget{
			MaxDuration:              5 * time.Minute,
			MaxCapabilityInvocations: 100,
		},
	}
}

// ConfigFromEnv creates a Config by reading environment variables,
// falling back to defaults for unset values.
//
// Environment variables:
//
//	AXI_ADDR                  - Server listen address (default ":8080")
//	AXI_READ_TIMEOUT_SECS    - HTTP read timeout in seconds (default 15)
//	AXI_WRITE_TIMEOUT_SECS   - HTTP write timeout in seconds (default 30)
//	AXI_IDLE_TIMEOUT_SECS    - HTTP idle timeout in seconds (default 60)
//	AXI_MAX_DURATION_SECS    - Max execution duration in seconds (default 300)
//	AXI_MAX_INVOCATIONS      - Max capability invocations per session (default 100)
func ConfigFromEnv() Config {
	cfg := DefaultConfig()

	if v := os.Getenv("AXI_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := envInt("AXI_READ_TIMEOUT_SECS"); v > 0 {
		cfg.ReadTimeout = time.Duration(v) * time.Second
	}
	if v := envInt("AXI_WRITE_TIMEOUT_SECS"); v > 0 {
		cfg.WriteTimeout = time.Duration(v) * time.Second
	}
	if v := envInt("AXI_IDLE_TIMEOUT_SECS"); v > 0 {
		cfg.IdleTimeout = time.Duration(v) * time.Second
	}
	if v := envInt("AXI_MAX_DURATION_SECS"); v > 0 {
		cfg.DefaultBudget.MaxDuration = time.Duration(v) * time.Second
	}
	if v := envInt("AXI_MAX_INVOCATIONS"); v > 0 {
		cfg.DefaultBudget.MaxCapabilityInvocations = v
	}

	return cfg
}

func envInt(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
