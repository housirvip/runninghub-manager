package apps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// VideoSplitApp splits a video into equal segments using ffmpeg.
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
			FieldName:  "segments",
			FieldType:  "INT",
			FieldValue: "2",
			FieldData:  nil,
		},
	}
}

func (a *VideoSplitApp) Execute(ctx context.Context, input AppInput) (*AppResult, error) {
	// Extract params from nodeInfoList
	var videoFile string
	var segments int

	for _, node := range input.NodeInfoList {
		switch node.FieldName {
		case "video":
			videoFile = node.FieldValue // filename in uploads dir
		case "segments":
			n, err := strconv.Atoi(node.FieldValue)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("invalid segments value: %s", node.FieldValue)
			}
			segments = n
		}
	}

	if videoFile == "" {
		return nil, fmt.Errorf("missing required field: video")
	}
	if segments == 0 {
		segments = 2 // default
	}

	inputPath := filepath.Join(input.UploadDir, videoFile)

	// Verify input file exists
	if _, err := os.Stat(inputPath); err != nil {
		return nil, fmt.Errorf("video file not found: %s", videoFile)
	}

	// Get video duration via ffprobe
	duration, err := getVideoDuration(ctx, inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video duration: %w", err)
	}

	if duration <= 0 {
		return nil, fmt.Errorf("invalid video duration: %.3f", duration)
	}

	segmentDuration := duration / float64(segments)
	ext := filepath.Ext(videoFile)

	var results []FileResult
	for i := range segments {
		startTime := float64(i) * segmentDuration
		outName := fmt.Sprintf("segment_%03d%s", i+1, ext)
		outPath := filepath.Join(input.OutputDir, outName)

		args := []string{
			"-y", "-i", inputPath,
			"-ss", fmt.Sprintf("%.3f", startTime),
			"-t", fmt.Sprintf("%.3f", segmentDuration),
			"-c", "copy",
			outPath,
		}
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("ffmpeg segment %d failed: %w\noutput: %s", i+1, err, string(output))
		}

		// Get file size
		var fileSize int64
		if info, err := os.Stat(outPath); err == nil {
			fileSize = info.Size()
		}

		// Build relative path for URL: task-{id}/segment_001.mp4
		relPath := filepath.Base(filepath.Dir(input.OutputDir+"/")) + "/" + outName
		// OutputDir is like ./output/task-123, so relPath = task-123/segment_001.mp4
		relPath = filepath.Base(input.OutputDir) + "/" + outName
		url := fmt.Sprintf("%s/output/%s", input.BaseURL, relPath)

		results = append(results, FileResult{
			URL:      url,
			Filename: outName,
			Size:     fileSize,
		})
	}

	return &AppResult{Files: results}, nil
}

func getVideoDuration(ctx context.Context, path string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}
	return strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
}
