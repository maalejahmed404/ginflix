package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	kcHost     = os.Getenv("KEYCLOAK_HOST")
	kcRealm    = os.Getenv("KEYCLOAK_REALM")
	kcClientId = os.Getenv("KEYCLOAK_CLIENT_ID")
)

var (
	videoCol         *mongo.Collection
	keycloakIssuer   = "https://" + kcHost + "/realms/" + kcRealm // no trailing slash
	keycloakJwksURL  = keycloakIssuer + "/protocol/openid-connect/certs"
	expectedAudience = "account"
)

func withCORS(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin) // Use specific origin instead of wildcard
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		handler(w, r)
	}
}

func main() {
	// Configure zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	// Use pretty console logging for development
	if os.Getenv("ENV") != "production" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}

	// err := godotenv.Load()
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("Error loading .env file")
	// }

	mongoURI := os.Getenv("MONGO_URI")
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal().Err(err).Msg("MongoDB connection failed")
	}
	videoCol = client.Database("videodb").Collection("videos")

	http.HandleFunc("/api/videos", withCORS(handleListVideos))
	http.HandleFunc("/api/video", withCORS(tokenMiddleware(handleUploadVideo)))
	http.HandleFunc("/api/video/process", withCORS(tokenMiddleware(handleProcessVideo)))
	http.HandleFunc("/api/video/status", withCORS(handleVideoStatus))
	http.HandleFunc("/api/video/delete", withCORS(tokenMiddleware(handleDeleteVideo)))
	http.HandleFunc("/api/video/rate", withCORS(handleRateVideo))

	log.Info().Msg("Server running on :8080")
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      nil,
		ReadTimeout:  2 * time.Minute,
		WriteTimeout: 2 * time.Minute,
		IdleTimeout:  1 * time.Minute,
	}
	log.Fatal().Err(srv.ListenAndServe()).Msg("Server failed")
}
