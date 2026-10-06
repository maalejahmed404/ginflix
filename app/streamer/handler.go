package main

import (
	"io"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/rs/zerolog/log"
)

func StreamHandler(client *minio.Client, cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger := log.With().
			Str("handler", "StreamHandler").
			Str("remote_addr", r.RemoteAddr).
			Logger()

		key := r.URL.Query().Get("file")
		if key == "" {
			logger.Warn().Msg("Missing 'file' query parameter")
			http.Error(w, "Missing 'file' query parameter", http.StatusBadRequest)
			return
		}

		logger.Debug().Str("file", key).Msg("Retrieving file from S3")
		obj, err := GetObjectStream(client, cfg.Bucket, key)
		if err != nil {
			logger.Error().Err(err).Str("file", key).Msg("Failed to retrieve file from S3")
			http.Error(w, "Failed to retrieve file", http.StatusBadGateway)
			return
		}
		defer obj.Close()

		logger.Debug().
			Str("file", key).
			Str("content_type", "application/octet-stream").
			Msg("Streaming file to client")

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		bytesWritten, err := io.Copy(w, obj)
		if err != nil {
			logger.Error().Err(err).Str("file", key).Msg("Error streaming file to client")
			return
		}

		logger.Info().
			Str("file", key).
			Int64("bytes_written", bytesWritten).
			Msg("Successfully streamed file to client")
	}
}
