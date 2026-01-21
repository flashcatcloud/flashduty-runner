package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/lithammer/shortuuid/v4"
)

const (
	// DefaultMaxOutputSize is the default character limit before truncation (~7.5k tokens)
	DefaultMaxOutputSize = 30000 // ~7.5k tokens at 4 chars/token
	// DefaultPreviewSize is the maximum characters for preview
	DefaultPreviewSize = 8000 // ~2k tokens
	// DefaultPreviewLines is the number of lines shown in the preview
	DefaultPreviewLines = 20
	// OutputsDir is the directory name for storing large outputs
	OutputsDir = ".work/outputs"
	// WorkDir is the base working directory
	WorkDir = ".work"
)

// LargeOutputConfig holds configuration for large output handling.
type LargeOutputConfig struct {
	MaxOutputSize int
	PreviewSize   int
	PreviewLines  int
}

// DefaultLargeOutputConfig returns the default configuration.
func DefaultLargeOutputConfig() LargeOutputConfig {
	return LargeOutputConfig{
		MaxOutputSize: DefaultMaxOutputSize,
		PreviewSize:   DefaultPreviewSize,
		PreviewLines:  DefaultPreviewLines,
	}
}

// LargeOutputProcessor handles large output truncation and file storage.
type LargeOutputProcessor struct {
	config LargeOutputConfig
	ws     *Workspace
}

// NewLargeOutputProcessor creates a new processor with the given workspace.
func NewLargeOutputProcessor(ws *Workspace, config LargeOutputConfig) *LargeOutputProcessor {
	if config.MaxOutputSize <= 0 {
		config.MaxOutputSize = DefaultMaxOutputSize
	}
	if config.PreviewSize <= 0 {
		config.PreviewSize = DefaultPreviewSize
	}
	if config.PreviewLines <= 0 {
		config.PreviewLines = DefaultPreviewLines
	}
	return &LargeOutputProcessor{
		config: config,
		ws:     ws,
	}
}

// ProcessResult holds the result of processing large output.
type ProcessResult struct {
	Content   string
	Truncated bool
	FilePath  string
	TotalSize int64
}

// Process checks if content exceeds the limit and handles accordingly.
func (p *LargeOutputProcessor) Process(ctx context.Context, content string, prefix string) (*ProcessResult, error) {
	totalSize := int64(len(content))

	// Content is within limit, return unchanged
	if len(content) <= p.config.MaxOutputSize {
		return &ProcessResult{
			Content:   content,
			Truncated: false,
			TotalSize: totalSize,
		}, nil
	}

	// Generate unique filename
	filename := fmt.Sprintf("%s_%s_%d.txt", prefix, shortuuid.New()[:8], time.Now().Unix())
	filePath := filepath.Join(OutputsDir, filename)

	// Save full content to file
	if err := p.ws.WriteRaw(ctx, filePath, []byte(content)); err != nil {
		// If save fails, just return truncated content without file reference
		return &ProcessResult{
			Content:   p.truncateContent(content, ""),
			Truncated: true,
			TotalSize: totalSize,
		}, nil
	}

	return &ProcessResult{
		Content:   p.truncateContent(content, filePath),
		Truncated: true,
		FilePath:  filePath,
		TotalSize: totalSize,
	}, nil
}

// ShouldSkipForWorkDir checks if large output processing should be skipped
// for commands operating on .work/ directory to avoid circular processing.
func ShouldSkipForWorkDir(command string) bool {
	readCommands := []string{"cat ", "head ", "tail ", "less ", "more ", "bat "}
	for _, cmd := range readCommands {
		if strings.Contains(command, cmd) {
			if strings.Contains(command, WorkDir+"/") || strings.Contains(command, WorkDir+"\\") {
				return true
			}
		}
	}
	return false
}

// truncateContent creates a truncated preview with optional file reference.
func (p *LargeOutputProcessor) truncateContent(content string, filePath string) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Calculate preview
	previewLines := min(p.config.PreviewLines, totalLines)
	preview := strings.Join(lines[:previewLines], "\n")

	// Truncate preview if too long
	if len(preview) > p.config.PreviewSize {
		preview = preview[:p.config.PreviewSize] + "\n... [preview truncated]"
	}

	// Build truncation message
	var sb strings.Builder
	sb.WriteString("<output_truncated>\n")
	sb.WriteString(fmt.Sprintf("Output too large (%d chars, %d lines).", len(content), totalLines))

	if filePath != "" {
		sb.WriteString(fmt.Sprintf(" Full content saved to: %s\n\n", filePath))
	} else {
		sb.WriteString(" Could not save full content.\n\n")
	}

	sb.WriteString(fmt.Sprintf("Preview (first %d lines):\n```\n%s\n```\n\n", previewLines, preview))

	if filePath != "" {
		sb.WriteString(fmt.Sprintf("To read more: read(\"%s\", offset=%d, limit=100)\n", filePath, previewLines))
	}

	sb.WriteString("</output_truncated>")
	return sb.String()
}
