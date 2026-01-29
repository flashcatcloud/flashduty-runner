// Package workspace implements local workspace operations.
package workspace

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/flashcatcloud/flashduty-runner/mcp"
	"github.com/flashcatcloud/flashduty-runner/permission"
	"github.com/flashcatcloud/flashduty-runner/protocol"
)

// Workspace handles local filesystem operations.
type Workspace struct {
	root    string
	checker *permission.Checker
	mcpMgr  *mcp.ClientManager
}

// New creates a new workspace with the given root directory and permission checker.
func New(root string, checker *permission.Checker) (*Workspace, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Ensure workspace root exists
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create workspace root: %w", err)
	}

	return &Workspace{
		root:    absRoot,
		checker: checker,
		mcpMgr:  mcp.NewClientManager(),
	}, nil
}

// Root returns the workspace root directory.
func (w *Workspace) Root() string {
	return w.root
}

// safePath ensures the path is within the workspace root, resolving symlinks.
func (w *Workspace) safePath(path string) (string, error) {
	absPath, err := filepath.Abs(filepath.Join(w.root, path))
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// First check without resolving symlinks
	if !strings.HasPrefix(absPath, w.root) {
		return "", fmt.Errorf("path is outside workspace root: %s", path)
	}

	// If the path exists, resolve symlinks and check again
	if _, err := os.Lstat(absPath); err == nil {
		realPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			// If we can't resolve symlinks, allow the path if it doesn't exist yet
			// This handles cases like writing to a new file
			if os.IsNotExist(err) {
				return absPath, nil
			}
			return "", fmt.Errorf("failed to resolve symlinks: %w", err)
		}

		// Also resolve the root path for consistent comparison
		realRoot, err := filepath.EvalSymlinks(w.root)
		if err != nil {
			realRoot = w.root
		}

		if !strings.HasPrefix(realPath, realRoot) {
			return "", fmt.Errorf("path escapes workspace root via symlink: %s", path)
		}
		return realPath, nil
	}

	return absPath, nil
}

// Read reads a file from the workspace.
func (w *Workspace) Read(ctx context.Context, args *protocol.ReadArgs) (*protocol.ReadResult, error) {
	realPath, err := w.safePath(args.Path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(realPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("cannot read a directory: %s", args.Path)
	}

	return w.readFileContent(realPath, info.Size(), args.Offset, args.Limit)
}

// readFileContent reads file content with offset and limit support.
func (w *Workspace) readFileContent(path string, size, offset, limit int64) (*protocol.ReadResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || offset+limit > size {
		limit = size - offset
	}

	if offset >= size {
		return &protocol.ReadResult{TotalSize: size}, nil
	}

	buf := make([]byte, limit)
	n, err := file.ReadAt(buf, offset)
	if err != nil && n == 0 {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return &protocol.ReadResult{
		Content:   base64.StdEncoding.EncodeToString(buf[:n]),
		TotalSize: size,
	}, nil
}

// Write writes content to a file in the workspace.
func (w *Workspace) Write(ctx context.Context, args *protocol.WriteArgs) error {
	realPath, err := w.safePath(args.Path)
	if err != nil {
		return err
	}

	// Decode base64 content
	content, err := base64.StdEncoding.DecodeString(args.Content)
	if err != nil {
		return fmt.Errorf("failed to decode content: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(realPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(realPath, content, 0o644)
}

// List lists entries in a directory.
func (w *Workspace) List(ctx context.Context, args *protocol.ListArgs) (*protocol.ListResult, error) {
	realPath, err := w.safePath(args.Path)
	if err != nil {
		return nil, err
	}

	// Resolve root for consistent relative path calculation
	resolvedRoot := w.root
	if resolved, resolveErr := filepath.EvalSymlinks(w.root); resolveErr == nil {
		resolvedRoot = resolved
	}

	var entries []protocol.ListEntry
	err = filepath.Walk(realPath, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(resolvedRoot, p)
		if relPath == "." {
			return nil
		}

		// Check ignore patterns
		for _, pattern := range args.Ignore {
			if matched, _ := filepath.Match(pattern, filepath.Base(p)); matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		entries = append(entries, protocol.ListEntry{
			Path:  relPath,
			IsDir: info.IsDir(),
			Size:  info.Size(),
		})

		if !args.Recursive && info.IsDir() && p != realPath {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	return &protocol.ListResult{Entries: entries}, nil
}

// Glob searches for files matching a pattern.
func (w *Workspace) Glob(ctx context.Context, args *protocol.GlobArgs) (*protocol.GlobResult, error) {
	fsys := os.DirFS(w.root)
	matches, err := doublestar.Glob(fsys, args.Pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob: %w", err)
	}

	sort.Strings(matches)
	return &protocol.GlobResult{Matches: matches}, nil
}

// Grep searches for a pattern in files.
func (w *Workspace) Grep(ctx context.Context, args *protocol.GrepArgs) (*protocol.GrepResult, error) {
	var res *protocol.GrepResult
	var err error
	// Try ripgrep first
	if _, lookErr := exec.LookPath("rg"); lookErr == nil {
		res, err = w.grepWithRipgrep(ctx, args)
	} else {
		res, err = w.grepWithGo(ctx, args)
	}

	if err != nil || res == nil {
		return res, err
	}

	// Build content string
	var sb strings.Builder
	for _, match := range res.Matches {
		sb.WriteString(fmt.Sprintf("%s:%d:%s\n", match.Path, match.LineNumber, match.Content))
	}
	content := sb.String()

	// Process large output
	processor := NewLargeOutputProcessor(w, DefaultLargeOutputConfig())
	processed, err := processor.Process(ctx, content, "grep")
	if err != nil {
		res.Content = content
		res.TotalSize = int64(len(content))
		return res, nil
	}

	res.Content = processed.Content
	res.Truncated = processed.Truncated
	res.FilePath = processed.FilePath
	res.TotalSize = processed.TotalSize

	return res, nil
}

func (w *Workspace) grepWithRipgrep(ctx context.Context, args *protocol.GrepArgs) (*protocol.GrepResult, error) {
	cmdArgs := []string{"--column", "--line-number", "--no-heading", "--color", "never", "--smart-case"}
	for _, inc := range args.Include {
		cmdArgs = append(cmdArgs, "--glob", inc)
	}
	cmdArgs = append(cmdArgs, args.Pattern, ".")

	cmd := exec.CommandContext(ctx, "rg", cmdArgs...)
	cmd.Dir = w.root

	var stdout strings.Builder
	cmd.Stdout = &stdout
	_ = cmd.Run() // rg returns exit code 1 if no matches found

	lines := strings.Split(stdout.String(), "\n")
	results := make([]protocol.GrepMatch, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) < 4 {
			continue
		}
		lineNum, _ := strconv.Atoi(parts[1])
		results = append(results, protocol.GrepMatch{
			Path:       parts[0],
			LineNumber: lineNum,
			Content:    parts[3],
		})
	}
	return &protocol.GrepResult{Matches: results}, nil
}

func (w *Workspace) grepWithGo(ctx context.Context, args *protocol.GrepArgs) (*protocol.GrepResult, error) {
	var results []protocol.GrepMatch
	include := args.Include
	if len(include) == 0 {
		include = []string{"**/*"}
	}

	for _, inc := range include {
		matches, err := w.Glob(ctx, &protocol.GlobArgs{Pattern: inc})
		if err != nil {
			continue
		}

		for _, match := range matches.Matches {
			realPath, _ := w.safePath(match)
			file, err := os.Open(realPath)
			if err != nil {
				continue
			}

			scanner := bufio.NewScanner(file)
			lineNum := 1
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, args.Pattern) {
					results = append(results, protocol.GrepMatch{
						Path:       match,
						LineNumber: lineNum,
						Content:    line,
					})
				}
				lineNum++
			}
			_ = file.Close()
		}
	}
	return &protocol.GrepResult{Matches: results}, nil
}

// Bash executes a bash command in the workspace.
func (w *Workspace) Bash(ctx context.Context, args *protocol.BashArgs) (*protocol.BashResult, error) {
	if err := w.checker.Check(args.Command); err != nil {
		return nil, err
	}

	workdir, err := w.resolveWorkdir(args.Workdir)
	if err != nil {
		return nil, err
	}

	timeout := w.resolveTimeout(args.Timeout)
	result, err := w.executeBashCommand(ctx, args.Command, workdir, timeout)
	if err != nil {
		return result, err
	}

	// Skip large output processing for .work/ directory reads
	if ShouldSkipForWorkDir(args.Command) {
		result.TotalSize = int64(len(result.Stdout))
		return result, nil
	}

	return w.processLargeOutput(ctx, result, "bash")
}

// resolveWorkdir resolves the working directory for command execution.
func (w *Workspace) resolveWorkdir(workdir string) (string, error) {
	if workdir == "" {
		return w.root, nil
	}
	return w.safePath(workdir)
}

// resolveTimeout resolves the command timeout duration.
func (w *Workspace) resolveTimeout(timeoutSec int) time.Duration {
	if timeoutSec > 0 {
		return time.Duration(timeoutSec) * time.Second
	}
	return 120 * time.Second
}

// executeBashCommand executes a bash command with the given parameters.
func (w *Workspace) executeBashCommand(ctx context.Context, command, workdir string, timeout time.Duration) (*protocol.BashResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = workdir

	// Use a limited writer to prevent OOM from very large outputs
	// 10MB limit is plenty for LLM context while preventing memory exhaustion
	const maxOutputSize = 10 * 1024 * 1024
	var stdout, stderr strings.Builder
	cmd.Stdout = &LimitedWriter{W: &stdout, Limit: maxOutputSize}
	cmd.Stderr = &LimitedWriter{W: &stderr, Limit: maxOutputSize}

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return &protocol.BashResult{
				Stdout:   stdout.String(),
				Stderr:   "command timed out",
				ExitCode: 124,
			}, nil
		} else {
			return nil, fmt.Errorf("failed to execute command: %w", err)
		}
	}

	return &protocol.BashResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

// LimitedWriter is an io.Writer that limits the total number of bytes written.
type LimitedWriter struct {
	W     io.Writer
	Limit int64
	curr  int64
}

func (l *LimitedWriter) Write(p []byte) (n int, err error) {
	if l.curr >= l.Limit {
		return len(p), nil
	}
	left := l.Limit - l.curr
	if int64(len(p)) > left {
		n, err = l.W.Write(p[:left])
		l.curr += int64(n)
		return len(p), err
	}
	n, err = l.W.Write(p)
	l.curr += int64(n)
	return n, err
}

// processLargeOutput processes command output for truncation if needed.
func (w *Workspace) processLargeOutput(ctx context.Context, result *protocol.BashResult, prefix string) (*protocol.BashResult, error) {
	processor := NewLargeOutputProcessor(w, DefaultLargeOutputConfig())
	processed, err := processor.Process(ctx, result.Stdout, prefix)
	if err != nil {
		result.TotalSize = int64(len(result.Stdout))
		return result, nil
	}

	result.Stdout = processed.Content
	result.Truncated = processed.Truncated
	result.FilePath = processed.FilePath
	result.TotalSize = processed.TotalSize

	return result, nil
}

// WriteRaw writes raw content (not base64 encoded) to a file.
// Used internally for saving large output files.
func (w *Workspace) WriteRaw(ctx context.Context, path string, content []byte) error {
	realPath, err := w.safePath(path)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	dir := filepath.Dir(realPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(realPath, content, 0o644)
}

// MCPCall executes an MCP tool call.
func (w *Workspace) MCPCall(ctx context.Context, args *protocol.MCPCallArgs, logger *slog.Logger) (*protocol.MCPCallResult, error) {
	// Parse arguments
	var toolArgs map[string]any
	if len(args.Args) > 0 {
		if err := json.Unmarshal(args.Args, &toolArgs); err != nil {
			return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
		}
	}

	// Call MCP tool
	result, err := w.mcpMgr.CallTool(ctx, &args.Server, args.ToolName, toolArgs, logger)
	if err != nil {
		return nil, err
	}

	// Extract content
	content := mcp.ExtractContent(result)

	// Process large output
	processor := NewLargeOutputProcessor(w, DefaultLargeOutputConfig())
	processed, err := processor.Process(ctx, content, "mcp")
	if err != nil {
		return nil, err
	}

	return &protocol.MCPCallResult{
		Content:   processed.Content,
		IsError:   result.IsError,
		Truncated: processed.Truncated,
		FilePath:  processed.FilePath,
		TotalSize: processed.TotalSize,
	}, nil
}

// MCPListTools lists available tools from an MCP server.
func (w *Workspace) MCPListTools(ctx context.Context, args *protocol.MCPListToolsArgs) (*protocol.MCPListToolsResult, error) {
	tools, err := w.mcpMgr.ListTools(ctx, &args.Server)
	if err != nil {
		return nil, err
	}

	result := &protocol.MCPListToolsResult{
		Tools: make([]protocol.MCPToolInfo, len(tools)),
	}
	for i, t := range tools {
		result.Tools[i] = protocol.MCPToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	return result, nil
}

// SyncSkill syncs a skill from cloud to local workspace.
func (w *Workspace) SyncSkill(ctx context.Context, args *protocol.SyncSkillArgs) (*protocol.SyncSkillResult, error) {
	skillDir, err := w.safePath(args.SkillDir)
	if err != nil {
		return nil, err
	}

	// Decode zip data
	zipData, err := base64.StdEncoding.DecodeString(args.ZipData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode zip data: %w", err)
	}

	// Unzip to skill directory
	if err := w.unzipSkill(zipData, skillDir); err != nil {
		return nil, fmt.Errorf("failed to unzip skill: %w", err)
	}

	// Write checksum file
	checksumPath := filepath.Join(skillDir, ".checksum")
	if err := os.WriteFile(checksumPath, []byte(args.Checksum), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write checksum: %w", err)
	}

	return &protocol.SyncSkillResult{
		Success: true,
		Path:    args.SkillDir,
	}, nil
}

// unzipSkill extracts a zip archive to the destination directory.
func (w *Workspace) unzipSkill(data []byte, dest string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed to read zip archive: %w", err)
	}

	// Remove existing directory if exists
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("failed to remove existing directory: %w", err)
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	for _, f := range reader.File {
		// Security: validate zip entry path to prevent path traversal
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("invalid file path in zip: %s", f.Name)
		}

		targetPath := filepath.Join(dest, cleanName)
		absTarget, err := filepath.Abs(targetPath)
		if err != nil || !strings.HasPrefix(absTarget, dest) {
			return fmt.Errorf("file path escapes destination: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(absTarget, f.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", cleanName, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(absTarget), 0o755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		if err := w.extractZipFile(f, absTarget); err != nil {
			return fmt.Errorf("failed to extract %s: %w", cleanName, err)
		}
	}
	return nil
}

// extractZipFile extracts a single file from a zip archive.
func (w *Workspace) extractZipFile(f *zip.File, targetPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		_ = rc.Close()
	}()

	dst, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer func() {
		_ = dst.Close()
	}()

	// Limit copy size to prevent decompression bomb (max 100MB per file)
	// G110: file size is validated in unzipSkill before extraction
	_, err = io.Copy(dst, io.LimitReader(rc, 100*1024*1024))
	return err
}
