package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"
)

func uploadHLSToS3(dir string, baseName string) (string, error) {
	logger := log.With().
		Str("function", "uploadHLSToS3").
		Str("dir", dir).
		Str("base_name", baseName).
		Logger()

	logger.Debug().Msg("Starting HLS upload to S3")

	endpoint := os.Getenv("GARAGE_ENDPOINT")
	accessKey := os.Getenv("GARAGE_ACCESS_KEY")
	secretKey := os.Getenv("GARAGE_SECRET_KEY")
	bucket := os.Getenv("GARAGE_BUCKET")

	logger.Debug().
		Str("endpoint", endpoint).
		Str("bucket", bucket).
		Msg("Initializing S3 client")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
	})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize S3 client")
		return "", err
	}

	logger.Debug().Msg("Reading HLS directory")
	files, err := os.ReadDir(dir)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to read HLS directory")
		return "", err
	}

	prefix := "videos/" + baseName
	logger.Info().
		Int("file_count", len(files)).
		Str("prefix", prefix).
		Msg("Uploading HLS files to S3")

	uploadedCount := 0
	for _, file := range files {
		fileName := file.Name()
		filePath := filepath.Join(dir, fileName)
		s3Path := fmt.Sprintf("%s/%s", prefix, fileName)

		logger.Debug().
			Str("file", fileName).
			Str("s3_path", s3Path).
			Msg("Reading file for upload")

		content, err := os.ReadFile(filePath)
		if err != nil {
			logger.Error().
				Err(err).
				Str("file", filePath).
				Msg("Failed to read file for upload")
			return "", err
		}

		logger.Debug().
			Str("file", fileName).
			Int("size", len(content)).
			Msg("Uploading file to S3")

		_, err = client.PutObject(
			context.Background(), bucket,
			s3Path,
			bytes.NewReader(content), int64(len(content)),
			minio.PutObjectOptions{ContentType: "application/octet-stream"},
		)
		if err != nil {
			logger.Error().
				Err(err).
				Str("file", fileName).
				Str("s3_path", s3Path).
				Msg("Failed to upload file to S3")
			return "", err
		}
		uploadedCount++
	}

	hlsURL := fmt.Sprintf("%s/master.m3u8", prefix)
	logger.Info().
		Int("uploaded_files", uploadedCount).
		Str("hls_url", hlsURL).
		Msg("Successfully uploaded all HLS files to S3")

	return hlsURL, nil
}

func uploadThumbnailToS3(thumbnailPath, baseName string) (string, error) {
	logger := log.With().
		Str("function", "uploadThumbnailToS3").
		Str("thumbnail_path", thumbnailPath).
		Str("base_name", baseName).
		Logger()

	logger.Debug().Msg("Starting thumbnail upload to S3")

	endpoint := os.Getenv("GARAGE_ENDPOINT")
	accessKey := os.Getenv("GARAGE_ACCESS_KEY")
	secretKey := os.Getenv("GARAGE_SECRET_KEY")
	bucket := os.Getenv("GARAGE_BUCKET")

	logger.Debug().
		Str("endpoint", endpoint).
		Str("bucket", bucket).
		Msg("Initializing S3 client")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
	})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize S3 client")
		return "", err
	}

	logger.Debug().Msg("Reading thumbnail file")
	content, err := os.ReadFile(thumbnailPath)
	if err != nil {
		logger.Error().
			Err(err).
			Str("file", thumbnailPath).
			Msg("Failed to read thumbnail file")
		return "", err
	}

	s3Path := fmt.Sprintf("thumbnails/%s.jpg", baseName)
	logger.Debug().
		Str("s3_path", s3Path).
		Int("size", len(content)).
		Msg("Uploading thumbnail to S3")

	_, err = client.PutObject(
		context.Background(), bucket,
		s3Path,
		bytes.NewReader(content), int64(len(content)),
		minio.PutObjectOptions{ContentType: "image/jpeg"},
	)
	if err != nil {
		logger.Error().
			Err(err).
			Str("s3_path", s3Path).
			Msg("Failed to upload thumbnail to S3")
		return "", err
	}

	thumbnailURL := s3Path
	logger.Info().
		Str("thumbnail_url", thumbnailURL).
		Msg("Successfully uploaded thumbnail to S3")

	return thumbnailURL, nil
}

func deleteFromS3(hlsURL string) error {
	logger := log.With().
		Str("function", "deleteFromS3").
		Str("hls_url", hlsURL).
		Logger()

	logger.Debug().Msg("Starting deletion from S3")

	endpoint := os.Getenv("GARAGE_ENDPOINT")
	accessKey := os.Getenv("GARAGE_ACCESS_KEY")
	secretKey := os.Getenv("GARAGE_SECRET_KEY")
	bucket := os.Getenv("GARAGE_BUCKET")

	logger.Debug().
		Str("endpoint", endpoint).
		Str("bucket", bucket).
		Msg("Initializing S3 client")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
	})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize S3 client")
		return err
	}

	// Extract the prefix from the HLS URL (e.g., "videos/filename/index.m3u8" -> "videos/filename")
	prefix := strings.TrimSuffix(hlsURL, "/index.m3u8")
	logger.Debug().Str("prefix", prefix).Msg("Listing objects to delete")

	// List all objects with the prefix
	objectCh := client.ListObjects(context.Background(), bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	// Delete each object
	for object := range objectCh {
		if object.Err != nil {
			logger.Error().Err(object.Err).Msg("Error listing objects")
			return object.Err
		}

		logger.Debug().Str("object", object.Key).Msg("Deleting object")
		err := client.RemoveObject(context.Background(), bucket, object.Key, minio.RemoveObjectOptions{})
		if err != nil {
			logger.Error().Err(err).Str("object", object.Key).Msg("Failed to delete object")
			return err
		}
	}

	logger.Info().Str("prefix", prefix).Msg("Successfully deleted all objects with prefix")
	return nil
}
