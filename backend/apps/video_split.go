package apps

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// VideoSplitApp splits a video into segments using ffmpeg.
// Supports two modes: by segment count (equal split) or by fixed duration.
type VideoSplitApp struct{}

func init() {
	Register(&VideoSplitApp{})
}

func (a *VideoSplitApp) ID() string   { return "video-split" }
func (a *VideoSplitApp) Name() string { return "视频分割" }

func (a *VideoSplitApp) NodeInfoList() []NodeInfoSchema {
	return []NodeInfoSchema{
		{
			NodeID:     "1",
			FieldName:  "video",
			FieldType:  "FILE_PATH",
			FieldValue: "",
			FieldData:  nil,
		},
		{
			NodeID:     "2",
			FieldName:  "mode",
			FieldType:  "SELECT",
			FieldValue: "count",
			FieldData: []map[string]string{
				{"label": "按段数平均分割", "value": "count"},
				{"label": "按固定时长分割", "value": "duration"},
			},
		},
		{
			NodeID:     "3",
			FieldName:  "segments",
			FieldType:  "INT",
			FieldValue: "2",
			FieldData:  nil,
		},
		{
			NodeID:     "4",
			FieldName:  "duration",
			FieldType:  "FLOAT",
			FieldValue: "2",
			FieldData:  nil,
		},
	}
}

func (a *VideoSplitApp) Execute(ctx context.Context, input AppInput) (*AppResult, error) {
	var videoFile string
	var mode string
	var segments int
	var segDuration float64 // seconds per segment (fixed-duration mode)

	for _, node := range input.NodeInfoList {
		switch node.FieldName {
		case "video":
			videoFile = node.FieldValue
		case "mode":
			mode = node.FieldValue
		case "segments":
			if n, err := strconv.Atoi(node.FieldValue); err == nil && n > 0 {
				segments = n
			}
		case "duration":
			if f, err := strconv.ParseFloat(node.FieldValue, 64); err == nil && f > 0 {
				segDuration = f
			}
		}
	}

	if videoFile == "" {
		return nil, fmt.Errorf("missing required field: video")
	}

	// Sanitize filename to prevent path traversal
	videoFile = filepath.Base(videoFile)
	if videoFile == "." || videoFile == ".." {
		return nil, fmt.Errorf("invalid video filename")
	}

	if mode == "" {
		mode = "count"
	}

	inputPath := filepath.Join(input.UploadDir, videoFile)
	if _, err := os.Stat(inputPath); err != nil {
		return nil, fmt.Errorf("video file not found: %s", videoFile)
	}

	// Get total video duration
	totalDuration, err := getVideoDuration(ctx, inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video duration: %w", err)
	}
	if totalDuration <= 0 {
		return nil, fmt.Errorf("invalid video duration: %.3f", totalDuration)
	}

	// Determine segment count and per-segment duration based on mode
	var numSegments int
	var perSegDuration float64

	switch mode {
	case "duration":
		if segDuration <= 0 {
			segDuration = 2 // default 2s
		}
		perSegDuration = segDuration
		numSegments = int(math.Ceil(totalDuration / perSegDuration))
	default: // "count"
		if segments < 1 {
			segments = 2
		}
		numSegments = segments
		perSegDuration = totalDuration / float64(numSegments)
	}

	ext := filepath.Ext(videoFile)
	var results []FileResult

	for i := range numSegments {
		// Check context cancellation before spawning next ffmpeg process
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("task cancelled: %w", err)
		}

		startTime := float64(i) * perSegDuration

		// For the last segment, use remaining duration to avoid overshooting
		thisDuration := perSegDuration
		if remaining := totalDuration - startTime; remaining < thisDuration {
			thisDuration = remaining
		}
		if thisDuration <= 0 {
			break
		}

		outName := fmt.Sprintf("segment_%03d%s", i+1, ext)
		outPath := filepath.Join(input.OutputDir, outName)

		// -ss before -i for fast input seeking (key-frame level)
		args := []string{
			"-y",
			"-ss", fmt.Sprintf("%.3f", startTime),
			"-i", inputPath,
			"-t", fmt.Sprintf("%.3f", thisDuration),
			"-c", "copy",
			outPath,
		}
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("ffmpeg segment %d failed: %w\noutput: %s", i+1, err, string(output))
		}

		var fileSize int64
		if info, err := os.Stat(outPath); err == nil {
			fileSize = info.Size()
		}

		relPath := filepath.Base(input.OutputDir) + "/" + outName
		url := fmt.Sprintf("%s/files/%s", input.BaseURL, relPath)

		results = append(results, FileResult{
			URL:      url,
			Filename: outName,
			Size:     fileSize,
		})
	}

	return &AppResult{Files: results}, nil
}

func getVideoDuration(ctx context.Context, path string) (float64, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w (stderr: %s)", err, stderr.String())
	}
	return strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
}
