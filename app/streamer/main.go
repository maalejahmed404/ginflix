package main

import (
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	// Set global log level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Enable debug level if DEBUG environment variable is set
	if os.Getenv("DEBUG") == "true" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}

func withCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := log.With().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Logger()

		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin) // Use specific origin instead of wildcard
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			logger.Debug().Msg("Handling OPTIONS request")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		logger.Debug().Msg("Handling request")
		handler(w, r)
	}
}

func main() {
	log.Info().Msg("Starting streamer service")

	cfg := LoadConfig()
	log.Debug().
		Str("endpoint", cfg.Endpoint).
		Str("bucket", cfg.Bucket).
		Bool("use_ssl", cfg.UseSSL).
		Msg("Configuration loaded")

	client := InitS3Client(cfg)
	log.Debug().Msg("S3 client initialized")

	http.HandleFunc("/stream", withCORS(StreamHandler(client, cfg)))

	log.Info().Msg("Server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}
