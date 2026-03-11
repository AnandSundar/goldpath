package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Server configuration
	Port         string
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// Feature flags configuration
	FlagStorage string // "memory" or "redis"

	// Redis configuration
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// AI configuration
	AIEnabled    bool
	OpenAIAPIKey string

	// Observability
	MetricsEnabled bool
	LogLevel       string

	// SLO configuration
	SLOThreshold float64
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Port:         getEnv("GOLDPATH_PORT", "8080"),
		Host:         getEnv("GOLDPATH_HOST", "0.0.0.0"),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,

		FlagStorage: getEnv("GOLDPATH_FLAG_STORAGE", "memory"),

		RedisAddr:     getEnv("GOLDPATH_REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("GOLDPATH_REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("GOLDPATH_REDIS_DB", 0),

		AIEnabled:    getEnvBool("GOLDPATH_AI_ENABLED", false),
		OpenAIAPIKey: getEnv("GOLDPATH_OPENAI_API_KEY", ""),

		MetricsEnabled: getEnvBool("GOLDPATH_METRICS_ENABLED", true),
		LogLevel:       getEnv("GOLDPATH_LOG_LEVEL", "info"),

		SLOThreshold: getEnvFloat64("GOLDPATH_SLO_THRESHOLD", 0.99),
	}
}

// Load loads configuration from environment and optional config file
func Load(configFile string) (*Config, error) {
	cfg := DefaultConfig()

	// TODO: Support YAML config file if provided
	if configFile != "" {
		// Load from YAML file (future enhancement)
		_ = configFile // Placeholder for config file loading
	}

	return cfg, nil
}

// getEnv returns the value of an environment variable or a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvBool returns the boolean value of an environment variable
func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		lower := strings.ToLower(value)
		if lower == "true" || lower == "1" || lower == "yes" {
			return true
		}
		if lower == "false" || lower == "0" || lower == "no" {
			return false
		}
	}
	return defaultValue
}

// getEnvInt returns the integer value of an environment variable
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvFloat64 returns the float64 value of an environment variable
func getEnvFloat64(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}
