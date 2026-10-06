package main

import (
	"context"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"
)

func InitS3Client(cfg *Config) *minio.Client {
	logger := log.With().Str("function", "InitS3Client").Logger()
	logger.Debug().
		Str("endpoint", cfg.Endpoint).
		Bool("use_ssl", cfg.UseSSL).
		Msg("Initializing S3 client")

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize S3 client")
	}

	logger.Debug().Msg("S3 client initialized successfully")
	return client
}

func GetObjectStream(client *minio.Client, bucket, object string) (*minio.Object, error) {
	logger := log.With().
		Str("function", "GetObjectStream").
		Str("bucket", bucket).
		Str("object", object).
		Logger()

	logger.Debug().Msg("Getting object from S3")

	obj, err := client.GetObject(context.Background(), bucket, object, minio.GetObjectOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get object from S3")
		return nil, err
	}

	logger.Debug().Msg("Successfully got object from S3")
	return obj, nil
}
