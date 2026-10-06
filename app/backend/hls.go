package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

func saveTempFile(file io.Reader, filename string) (string, error) {
	logger := log.With().
		Str("function", "saveTempFile").
		Str("filename", filename).
		Logger()

	tmpPath := filepath.Join(os.TempDir(), filename)
	logger.Debug().Str("temp_path", tmpPath).Msg("Creating temporary file")

	out, err := os.Create(tmpPath)
	if err != nil {
		logger.Error().Err(err).Str("path", tmpPath).Msg("Failed to create temporary file")
		return "", err
	}
	defer out.Close()

	logger.Debug().Msg("Copying file data to temporary file")
	bytesWritten, err := io.Copy(out, file)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to copy data to temporary file")
		return "", err
	}

	logger.Debug().Int64("bytes_written", bytesWritten).Msg("File saved to temporary location")
	return tmpPath, nil
}

func convertToHLS(inputPath string) (string, error) {
	logger := log.With().
		Str("function", "convertToHLS").
		Str("input_path", inputPath).
		Logger()

	hlsDir := inputPath + "_hls"
	logger.Debug().Str("hls_dir", hlsDir).Msg("Creating HLS directory")

	if err := os.Mkdir(hlsDir, 0755); err != nil {
		logger.Error().Err(err).Str("dir", hlsDir).Msg("Failed to create HLS directory")
		return "", err
	}

	// Create master playlist file
	masterPlaylistPath := filepath.Join(hlsDir, "master.m3u8")
	logger.Debug().
		Str("master_playlist_path", masterPlaylistPath).
		Msg("Creating master playlist")

	// Define resolution variants
	variants := []struct {
		name       string
		resolution string
		bitrate    string
		maxrate    string
		bufsize    string
	}{
		{
			name:       "low",
			resolution: "640x360",
			bitrate:    "800k",
			maxrate:    "856k",
			bufsize:    "1200k",
		},
		{
			name:       "medium",
			resolution: "960x540",
			bitrate:    "1400k",
			maxrate:    "1498k",
			bufsize:    "2100k",
		},
		{
			name:       "high",
			resolution: "1280x720",
			bitrate:    "2800k",
			maxrate:    "2996k",
			bufsize:    "4200k",
		},
	}

	// Create master playlist content
	masterContent := "#EXTM3U\n#EXT-X-VERSION:3\n"
	for _, variant := range variants {
		masterContent += fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%s,RESOLUTION=%s\n%s.m3u8\n",
			strings.TrimSuffix(variant.bitrate, "k")+"000",
			variant.resolution,
			variant.name)
	}

	// Write master playlist to file
	err := os.WriteFile(masterPlaylistPath, []byte(masterContent), 0644)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to write master playlist")
		return "", err
	}

	// Create each variant
	for _, variant := range variants {
		variantOutputPath := filepath.Join(hlsDir, variant.name+".m3u8")
		logger.Debug().
			Str("variant", variant.name).
			Str("resolution", variant.resolution).
			Str("bitrate", variant.bitrate).
			Str("output_path", variantOutputPath).
			Msg("Creating HLS variant")

		cmd := exec.Command("ffmpeg",
			"-i", inputPath,
			"-profile:v", "main",
			"-level", "3.1",
			"-vf", fmt.Sprintf("scale=%s", variant.resolution),
			"-b:v", variant.bitrate,
			"-maxrate", variant.maxrate,
			"-bufsize", variant.bufsize,
			"-c:v", "h264",
			"-c:a", "aac",
			"-b:a", "128k",
			"-start_number", "0",
			"-hls_time", "10",
			"-hls_list_size", "0",
			"-f", "hls",
			variantOutputPath,
		)

		// Capture ffmpeg output for logging
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error().
				Err(err).
				Str("variant", variant.name).
				Str("ffmpeg_output", string(output)).
				Msg("ffmpeg conversion failed for variant")
			return "", err
		}

		logger.Debug().
			Str("variant", variant.name).
			Msg("Variant successfully created")
	}

	logger.Debug().Msg("Video successfully converted to multi-resolution HLS format")
	return hlsDir, nil
}

func removeTempFiles(path string) {
	logger := log.With().
		Str("function", "removeTempFiles").
		Str("path", path).
		Logger()

	logger.Debug().Msg("Removing temporary files")
	err := os.RemoveAll(path)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to remove temporary files")
	} else {
		logger.Debug().Msg("Temporary files removed successfully")
	}
}

func rewriteM3U8Playlist(playlistPath, fullFolder string) error {
	logger := log.With().
		Str("function", "rewriteM3U8Playlist").
		Str("playlist_path", playlistPath).
		Str("folder", fullFolder).
		Logger()

	logger.Debug().Msg("Reading M3U8 playlist file")
	input, err := os.ReadFile(playlistPath)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to read M3U8 file")
		return fmt.Errorf("failed to read m3u8 file: %w", err)
	}

	lines := strings.Split(string(input), "\n")

	// Check if this is a master playlist or a variant playlist
	isMaster := false
	for _, line := range lines {
		if strings.Contains(line, "#EXT-X-STREAM-INF") {
			isMaster = true
			break
		}
	}

	if isMaster {
		logger.Debug().Msg("Rewriting variant URLs in master playlist")
		for i, line := range lines {
			if strings.HasSuffix(line, ".m3u8") && !strings.Contains(line, "/stream?file=") {
				variant := filepath.Base(line)
				lines[i] = fmt.Sprintf("/stream?file=%s/%s", fullFolder, variant)
			}
		}
	} else {
		logger.Debug().Msg("Rewriting segment URLs in variant playlist")
		segmentCount := 0
		for i, line := range lines {
			if strings.HasSuffix(line, ".ts") {
				segment := filepath.Base(line)
				lines[i] = fmt.Sprintf("/stream?file=%s/%s", fullFolder, segment)
				segmentCount++
			}
		}
		logger.Debug().Int("segment_count", segmentCount).Msg("Segment URLs rewritten")
	}

	logger.Debug().Msg("Writing updated M3U8 playlist")
	err = os.WriteFile(playlistPath, []byte(strings.Join(lines, "\n")), 0644)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to write updated M3U8 file")
		return fmt.Errorf("failed to write m3u8 file: %w", err)
	}

	logger.Debug().Msg("M3U8 playlist successfully rewritten")
	return nil
}

func generateThumbnail(inputPath string) (string, error) {
	logger := log.With().
		Str("function", "generateThumbnail").
		Str("input_path", inputPath).
		Logger()

	thumbnailPath := inputPath + "_thumbnail.jpg"
	logger.Debug().Str("thumbnail_path", thumbnailPath).Msg("Generating thumbnail")

	// Use ffmpeg to extract a frame at 1 second as the thumbnail
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-ss", "00:00:01.000",
		"-vframes", "1",
		"-vf", "scale=320:-1",
		"-q:v", "2",
		thumbnailPath,
	)

	// Capture ffmpeg output for logging
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error().
			Err(err).
			Str("ffmpeg_output", string(output)).
			Msg("ffmpeg thumbnail generation failed")
		return "", err
	}

	logger.Debug().Str("thumbnail_path", thumbnailPath).Msg("Thumbnail successfully generated")
	return thumbnailPath, nil
}
