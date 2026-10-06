package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

func handleListVideos(w http.ResponseWriter, r *http.Request) {
	logger := log.With().
		Str("handler", "listVideos").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	logger.Info().Msg("Received request to list videos")

	if r.Method != http.MethodGet {
		logger.Warn().Str("expected_method", http.MethodGet).Msg("Method not allowed")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	videos, err := listVideos()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to fetch videos")
		http.Error(w, "Failed to fetch videos", http.StatusInternalServerError)
		return
	}

	logger.Info().Int("video_count", len(videos)).Msg("Successfully fetched videos")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(videos)
}

func handleUploadVideo(w http.ResponseWriter, r *http.Request) {
	logger := log.With().
		Str("handler", "uploadVideo").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	logger.Info().Msg("Received request to upload video")

	if r.Method != http.MethodPost {
		logger.Warn().Str("expected_method", http.MethodPost).Msg("Method not allowed")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(1024 << 20) // 1GB
	if err != nil {
		logger.Error().Err(err).Msg("Failed to parse multipart form")
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		logger.Error().Err(err).Msg("Missing or invalid video file")
		http.Error(w, "Missing video file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	logger.Info().
		Str("filename", header.Filename).
		Int64("size", header.Size).
		Msg("Uploading video")

	meta, err := uploadVideo(file, header.Filename)
	if err != nil {
		logger.Error().Err(err).Str("filename", header.Filename).Msg("Video upload failed")
		http.Error(w, "Video upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info().
		Str("video_id", meta.ID.Hex()).
		Str("title", meta.Title).
		Msg("Video successfully uploaded")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

func handleProcessVideo(w http.ResponseWriter, r *http.Request) {
	logger := log.With().
		Str("handler", "processVideo").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	logger.Info().Msg("Received request to process video")

	if r.Method != http.MethodPost {
		logger.Warn().Str("expected_method", http.MethodPost).Msg("Method not allowed")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var processRequest struct {
		VideoID string `json:"video_id"`
	}

	err := json.NewDecoder(r.Body).Decode(&processRequest)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to parse request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate video ID
	if processRequest.VideoID == "" {
		logger.Warn().Msg("Missing video ID")
		http.Error(w, "Missing video ID", http.StatusBadRequest)
		return
	}

	logger.Info().
		Str("video_id", processRequest.VideoID).
		Msg("Processing video")

	// Process the video
	meta, err := processVideo(processRequest.VideoID)
	if err != nil {
		logger.Error().Err(err).Str("video_id", processRequest.VideoID).Msg("Video processing failed")
		http.Error(w, "Video processing failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info().
		Str("video_id", meta.ID.Hex()).
		Str("title", meta.Title).
		Msg("Video successfully processed")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

func handleVideoStatus(w http.ResponseWriter, r *http.Request) {
	logger := log.With().
		Str("handler", "videoStatus").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	logger.Info().Msg("Received request to check video status")

	if r.Method != http.MethodGet {
		logger.Warn().Str("expected_method", http.MethodGet).Msg("Method not allowed")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get video ID from query parameter
	videoID := r.URL.Query().Get("id")
	if videoID == "" {
		logger.Warn().Msg("Missing video ID")
		http.Error(w, "Missing video ID", http.StatusBadRequest)
		return
	}

	logger.Info().
		Str("video_id", videoID).
		Msg("Checking video status")

	// Get the video status
	status, err := getVideoStatus(videoID)
	if err != nil {
		logger.Error().Err(err).Str("video_id", videoID).Msg("Failed to get video status")
		http.Error(w, "Failed to get video status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info().
		Str("video_id", status.ID).
		Str("status", status.Status).
		Bool("processed", status.Processed).
		Msg("Video status retrieved")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func handleRateVideo(w http.ResponseWriter, r *http.Request) {
	logger := log.With().
		Str("handler", "rateVideo").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	logger.Info().Msg("Received request to rate video")

	if r.Method != http.MethodPost {
		logger.Warn().Str("expected_method", http.MethodPost).Msg("Method not allowed")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var ratingRequest struct {
		VideoID string `json:"video_id"`
		Rating  int    `json:"rating"`
	}

	err := json.NewDecoder(r.Body).Decode(&ratingRequest)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to parse request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the rating value (1-5)
	if ratingRequest.Rating < 1 || ratingRequest.Rating > 5 {
		logger.Warn().Int("rating", ratingRequest.Rating).Msg("Invalid rating value")
		http.Error(w, "Rating must be between 1 and 5", http.StatusBadRequest)
		return
	}

	// Validate video ID
	if ratingRequest.VideoID == "" {
		logger.Warn().Msg("Missing video ID")
		http.Error(w, "Missing video ID", http.StatusBadRequest)
		return
	}

	logger.Info().
		Str("video_id", ratingRequest.VideoID).
		Int("rating", ratingRequest.Rating).
		Msg("Rating video")

	// Get client IP address
	clientIP := r.RemoteAddr
	// If behind a proxy, try to get the real IP
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, use the first one
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			clientIP = strings.TrimSpace(ips[0])
		}
	}

	logger.Info().
		Str("client_ip", clientIP).
		Msg("Client IP address for rating")

	// Update the video rating
	err = rateVideo(ratingRequest.VideoID, ratingRequest.Rating, clientIP)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to rate video")
		http.Error(w, "Failed to rate video: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info().Msg("Video successfully rated")
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Video successfully rated"})
}

func handleDeleteVideo(w http.ResponseWriter, r *http.Request) {
	logger := log.With().
		Str("handler", "deleteVideo").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	logger.Info().Msg("Received request to delete video")

	if r.Method != http.MethodDelete {
		logger.Warn().Str("expected_method", http.MethodDelete).Msg("Method not allowed")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract video ID from URL query parameters
	videoID := r.URL.Query().Get("id")
	if videoID == "" {
		logger.Warn().Msg("Missing video ID")
		http.Error(w, "Missing video ID", http.StatusBadRequest)
		return
	}

	logger.Info().Str("video_id", videoID).Msg("Deleting video")
	err := deleteVideo(videoID)
	if err != nil {
		logger.Error().Err(err).Str("video_id", videoID).Msg("Failed to delete video")
		http.Error(w, "Failed to delete video: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info().Str("video_id", videoID).Msg("Video successfully deleted")
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Video successfully deleted"})
}
