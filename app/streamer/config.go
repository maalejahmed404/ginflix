package main

import (
	"os"

	"github.com/rs/zerolog/log"
)

type Config struct {
	Bucket    string
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

func LoadConfig() *Config {
	logger := log.With().Str("function", "LoadConfig").Logger()
	logger.Debug().Msg("Loading configuration from environment variables")

	// if err := godotenv.Load(); err != nil {
	// 	logger.Fatal().Err(err).Msg("Error loading .env file")
	// }

	config := &Config{
		Bucket:    mustGetEnv("GARAGE_BUCKET"),
		Endpoint:  mustGetEnv("GARAGE_ENDPOINT"),
		AccessKey: mustGetEnv("GARAGE_ACCESS_KEY"),
		SecretKey: mustGetEnv("GARAGE_SECRET_KEY"),
		UseSSL:    true,
	}

	logger.Debug().
		Str("bucket", config.Bucket).
		Str("endpoint", config.Endpoint).
		Bool("use_ssl", config.UseSSL).
		Msg("Configuration loaded successfully")

	return config
}

func mustGetEnv(key string) string {
	logger := log.With().
		Str("function", "mustGetEnv").
		Str("env_key", key).
		Logger()

	val := os.Getenv(key)
	if val == "" {
		logger.Fatal().Msgf("Missing required environment variable: %s", key)
	}

	// Don't log the actual value for security reasons, especially for keys and secrets
	logger.Debug().Msg("Environment variable found")
	return val
}
