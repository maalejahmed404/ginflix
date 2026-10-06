package main

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Video struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title        string             `json:"title"`
	CreatedAt    time.Time          `json:"created_at"`
	HLSURL       string             `json:"hls_url"`
	ThumbnailURL string             `json:"thumbnail_url"`
	Rating       float64            `json:"rating" bson:"rating,omitempty"`
	RatingCount  int                `json:"rating_count" bson:"rating_count,omitempty"`
	RatedIPs     map[string]int     `json:"-" bson:"rated_ips,omitempty"` // Map of IP addresses to their rating values
}

func listVideos() ([]Video, error) {
	logger := log.With().Str("function", "listVideos").Logger()
	logger.Debug().Msg("Fetching videos from database")

	// Only fetch processed videos
	cursor, err := videoCol.Find(context.Background(), primitive.M{"processed": true})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to query videos collection")
		return nil, err
	}

	// First try to decode as UploadedVideo structs
	var uploadedVideos []UploadedVideo
	if err := cursor.All(context.Background(), &uploadedVideos); err != nil {
		logger.Error().Err(err).Msg("Failed to decode videos from cursor")
		return nil, err
	}

	// Convert UploadedVideo structs to Video structs
	videos := make([]Video, 0, len(uploadedVideos))
	for _, uploadedVideo := range uploadedVideos {
		// Only include videos that have been processed and have valid URLs
		if uploadedVideo.Processed && uploadedVideo.HLSURL != "" && uploadedVideo.ThumbnailURL != "" {
			video := Video{
				ID:           uploadedVideo.ID,
				Title:        uploadedVideo.Title,
				CreatedAt:    uploadedVideo.CreatedAt,
				HLSURL:       uploadedVideo.HLSURL,
				ThumbnailURL: uploadedVideo.ThumbnailURL,
			}
			videos = append(videos, video)
		}
	}

	logger.Debug().Int("count", len(videos)).Msg("Successfully fetched videos from database")
	return videos, nil
}

func rateVideo(id string, rating int, ipAddress string) error {
	logger := log.With().
		Str("function", "rateVideo").
		Str("video_id", id).
		Int("rating", rating).
		Str("ip_address", ipAddress).
		Logger()

	logger.Debug().Msg("Converting string ID to ObjectID")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid ObjectID format")
		return err
	}

	logger.Debug().Msg("Finding video in database")
	var video Video
	err = videoCol.FindOne(context.Background(), primitive.M{"_id": objID}).Decode(&video)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find video")
		return err
	}

	// Initialize RatedIPs map if it doesn't exist
	if video.RatedIPs == nil {
		video.RatedIPs = make(map[string]int)
	}

	// Check if this IP has already rated the video
	oldRating, hasRated := video.RatedIPs[ipAddress]

	var newRating float64
	var newRatingCount int

	if hasRated {
		// IP has already rated, update the existing rating
		logger.Debug().
			Int("old_rating_from_ip", oldRating).
			Int("new_rating_from_ip", rating).
			Msg("Updating existing rating from IP")

		// Remove old rating and add new rating
		newRating = ((video.Rating * float64(video.RatingCount)) - float64(oldRating) + float64(rating)) / float64(video.RatingCount)
		newRatingCount = video.RatingCount
	} else {
		// New rating from this IP
		newRatingCount = video.RatingCount + 1
		newRating = ((video.Rating * float64(video.RatingCount)) + float64(rating)) / float64(newRatingCount)
	}

	// Store the IP's rating
	video.RatedIPs[ipAddress] = rating

	logger.Debug().
		Float64("old_rating", video.Rating).
		Int("old_count", video.RatingCount).
		Float64("new_rating", newRating).
		Int("new_count", newRatingCount).
		Msg("Updating video rating")

	// Update the video with new rating
	_, err = videoCol.UpdateOne(
		context.Background(),
		primitive.M{"_id": objID},
		primitive.M{
			"$set": primitive.M{
				"rating":       newRating,
				"rating_count": newRatingCount,
				"rated_ips":    video.RatedIPs,
			},
		},
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to update video rating")
		return err
	}

	logger.Info().
		Float64("new_rating", newRating).
		Int("rating_count", newRatingCount).
		Msg("Video rating updated successfully")
	return nil
}

func deleteVideo(id string) error {
	logger := log.With().
		Str("function", "deleteVideo").
		Str("video_id", id).
		Logger()

	logger.Debug().Msg("Converting string ID to ObjectID")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid ObjectID format")
		return err
	}

	logger.Debug().Msg("Finding video in database")
	var video Video
	err = videoCol.FindOne(context.Background(), primitive.M{"_id": objID}).Decode(&video)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find video")
		return err
	}

	logger.Debug().Str("hls_url", video.HLSURL).Msg("Deleting video files from S3")
	err = deleteFromS3(video.HLSURL)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to delete video files from S3")
		return err
	}

	// Delete the thumbnail as well
	if video.ThumbnailURL != "" {
		logger.Debug().Str("thumbnail_url", video.ThumbnailURL).Msg("Deleting thumbnail from S3")
		err = deleteFromS3(video.ThumbnailURL)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to delete thumbnail from S3")
			return err
		}
	}

	logger.Debug().Msg("Deleting video from database")
	_, err = videoCol.DeleteOne(context.Background(), primitive.M{"_id": objID})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to delete video from database")
		return err
	}

	logger.Info().Msg("Video successfully deleted")
	return nil
}

// UploadedVideo represents a video that has been uploaded but not yet processed
type UploadedVideo struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Title       string             `json:"title" bson:"title"`
	CreatedAt   time.Time          `json:"created_at" bson:"created_at"`
	TempPath    string             `json:"temp_path" bson:"temp_path"`
	Processed   bool               `json:"processed" bson:"processed"`
	HLSURL      string             `json:"hls_url,omitempty" bson:"hls_url,omitempty"`
	ThumbnailURL string            `json:"thumbnail_url,omitempty" bson:"thumbnail_url,omitempty"`
	Status      string             `json:"status" bson:"status"`
	Error       string             `json:"error,omitempty" bson:"error,omitempty"`
}

// ProcessingStatus represents the status of a video being processed
type ProcessingStatus struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"created_at"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	Processed   bool      `json:"processed"`
	HLSURL      string    `json:"hls_url,omitempty"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
}

func uploadVideo(file io.Reader, filename string) (*UploadedVideo, error) {
	logger := log.With().
		Str("function", "uploadVideo").
		Str("filename", filename).
		Logger()

	logger.Info().Msg("Starting video upload")

	tmpFilePath, err := saveTempFile(file, filename)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to save temporary file")
		return nil, err
	}
	logger.Debug().Str("temp_path", tmpFilePath).Msg("Temporary file saved")

	video := &UploadedVideo{
		ID:        primitive.NewObjectID(),
		Title:     filename,
		CreatedAt: time.Now(),
		TempPath:  tmpFilePath,
		Processed: false,
		Status:    "uploaded",
	}

	logger.Debug().Interface("video", video).Msg("Saving initial video metadata to database")
	res, err := videoCol.InsertOne(context.Background(), video)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to save video metadata to database")
		removeTempFiles(tmpFilePath)
		return nil, err
	}
	video.ID = res.InsertedID.(primitive.ObjectID)

	logger.Info().
		Str("video_id", video.ID.Hex()).
		Str("title", video.Title).
		Msg("Video upload completed successfully")

	// After successful upload, trigger processing in a goroutine
	go func() {
		logger.Info().Str("video_id", video.ID.Hex()).Msg("Starting automatic video processing")

		// Update status to processing
		_, err := videoCol.UpdateOne(
			context.Background(),
			primitive.M{"_id": video.ID},
			primitive.M{"$set": primitive.M{"status": "processing"}},
		)
		if err != nil {
			logger.Error().Err(err).Str("video_id", video.ID.Hex()).Msg("Failed to update video status")
		}

		_, err = processVideo(video.ID.Hex())
		if err != nil {
			logger.Error().Err(err).Str("video_id", video.ID.Hex()).Msg("Automatic video processing failed")

			// Update status to error
			_, updateErr := videoCol.UpdateOne(
				context.Background(),
				primitive.M{"_id": video.ID},
				primitive.M{"$set": primitive.M{
					"status": "error",
					"error":  err.Error(),
				}},
			)
			if updateErr != nil {
				logger.Error().Err(updateErr).Str("video_id", video.ID.Hex()).Msg("Failed to update video status")
			}
		} else {
			logger.Info().Str("video_id", video.ID.Hex()).Msg("Automatic video processing completed successfully")
		}
	}()

	return video, nil
}

func processVideo(videoID string) (*Video, error) {
	logger := log.With().
		Str("function", "processVideo").
		Str("video_id", videoID).
		Logger()

	logger.Info().Msg("Starting video processing")

	// Convert string ID to ObjectID
	objID, err := primitive.ObjectIDFromHex(videoID)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid video ID format")
		return nil, err
	}

	// Find the uploaded video in the database
	var uploadedVideo UploadedVideo
	err = videoCol.FindOne(context.Background(), primitive.M{"_id": objID}).Decode(&uploadedVideo)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find uploaded video")
		return nil, err
	}

	if uploadedVideo.Processed {
		logger.Warn().Msg("Video already processed")
		// Return the already processed video
		var video Video
		err = videoCol.FindOne(context.Background(), primitive.M{"_id": objID}).Decode(&video)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to find processed video")
			return nil, err
		}
		return &video, nil
	}

	tmpFilePath := uploadedVideo.TempPath
	filename := uploadedVideo.Title
	defer removeTempFiles(tmpFilePath)

	// Generate thumbnail from the video
	logger.Debug().Msg("Generating thumbnail")
	thumbnailPath, err := generateThumbnail(tmpFilePath)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to generate thumbnail")
		return nil, err
	}
	logger.Debug().Str("thumbnail_path", thumbnailPath).Msg("Thumbnail generated")

	// Upload thumbnail to S3
	logger.Debug().Msg("Uploading thumbnail to S3")
	baseFilename := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	thumbnailURL, err := uploadThumbnailToS3(thumbnailPath, baseFilename)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to upload thumbnail to S3")
		return nil, err
	}
	logger.Debug().Str("thumbnail_url", thumbnailURL).Msg("Thumbnail uploaded to S3")

	logger.Debug().Msg("Converting video to HLS format")
	hlsDir, err := convertToHLS(tmpFilePath)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to convert video to HLS format")
		return nil, err
	}
	logger.Debug().Str("hls_dir", hlsDir).Msg("Video converted to HLS format")

	prefix := "videos/" + filename // includes .mp4

	// Rewrite the master playlist
	masterPlaylistPath := filepath.Join(hlsDir, "master.m3u8")
	logger.Debug().
		Str("master_playlist_path", masterPlaylistPath).
		Str("prefix", prefix).
		Msg("Rewriting master M3U8 playlist")

	err = rewriteM3U8Playlist(masterPlaylistPath, prefix)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to rewrite master M3U8 playlist")
		return nil, err
	}

	// Rewrite each variant playlist
	variants := []string{"low", "medium", "high"}
	for _, variant := range variants {
		variantPlaylistPath := filepath.Join(hlsDir, variant+".m3u8")
		logger.Debug().
			Str("variant_playlist_path", variantPlaylistPath).
			Str("prefix", prefix).
			Msg("Rewriting variant M3U8 playlist")

		err = rewriteM3U8Playlist(variantPlaylistPath, prefix)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to rewrite variant M3U8 playlist")
			return nil, err
		}
	}

	logger.Debug().Msg("Uploading HLS files to S3")
	hlsURL, err := uploadHLSToS3(hlsDir, filename)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to upload HLS files to S3")
		return nil, err
	}
	logger.Debug().Str("hls_url", hlsURL).Msg("HLS files uploaded to S3")

	// Create the final video metadata
	video := &Video{
		ID:           objID,
		Title:        filename,
		CreatedAt:    uploadedVideo.CreatedAt,
		HLSURL:       hlsURL,
		ThumbnailURL: thumbnailURL,
	}

	// Update the UploadedVideo record with processed information
	_, err = videoCol.UpdateOne(
		context.Background(),
		primitive.M{"_id": objID},
		primitive.M{"$set": primitive.M{
			"processed":     true,
			"status":        "completed",
			"hls_url":       hlsURL,
			"thumbnail_url": thumbnailURL,
		}},
	)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to update video processing status")
		return nil, err
	}

	logger.Info().
		Str("video_id", video.ID.Hex()).
		Str("title", video.Title).
		Str("hls_url", video.HLSURL).
		Str("thumbnail_url", video.ThumbnailURL).
		Msg("Video processing completed successfully")

	return video, nil
}

// Get video processing status by ID
func getVideoStatus(videoID string) (*ProcessingStatus, error) {
	logger := log.With().
		Str("function", "getVideoStatus").
		Str("video_id", videoID).
		Logger()

	logger.Debug().Msg("Getting video processing status")

	// Convert string ID to ObjectID
	objID, err := primitive.ObjectIDFromHex(videoID)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid video ID format")
		return nil, err
	}

	// Find the video in the database
	var uploadedVideo UploadedVideo
	err = videoCol.FindOne(context.Background(), primitive.M{"_id": objID}).Decode(&uploadedVideo)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to find video")
		return nil, err
	}

	// Convert to ProcessingStatus
	status := &ProcessingStatus{
		ID:           uploadedVideo.ID.Hex(),
		Title:        uploadedVideo.Title,
		CreatedAt:    uploadedVideo.CreatedAt,
		Status:       uploadedVideo.Status,
		Error:        uploadedVideo.Error,
		Processed:    uploadedVideo.Processed,
		HLSURL:       uploadedVideo.HLSURL,
		ThumbnailURL: uploadedVideo.ThumbnailURL,
	}

	logger.Debug().
		Str("video_id", status.ID).
		Str("status", status.Status).
		Bool("processed", status.Processed).
		Msg("Video status retrieved")

	return status, nil
}

// This function has been removed as part of code cleanup
// The functionality is now handled by automatic processing in uploadVideo
