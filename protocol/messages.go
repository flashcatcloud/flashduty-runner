// Package protocol defines the WebSocket message protocol between Runner and Flashduty.
package protocol

import (
	"encoding/json"
	"time"

	"github.com/lithammer/shortuuid/v4"
)

// MessageType defines the type of WebSocket message.
type MessageType string

const (
	// Flashduty -> Runner (initial)
	MessageTypeWelcome MessageType = "welcome"

	// Runner -> Flashduty
	MessageTypeHeartbeat  MessageType = "heartbeat"
	MessageTypeTaskOutput MessageType = "task.output"
	MessageTypeTaskResult MessageType = "task.result"
	MessageTypeMCPResult  MessageType = "mcp.result"

	// Flashduty -> Runner
	MessageTypeTaskRequest MessageType = "task.request"
	MessageTypeTaskCancel  MessageType = "task.cancel"
	MessageTypeMCPCall     MessageType = "mcp.call"
)

// Message is the base WebSocket message structure.
type Message struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
}

// NewMessage creates a new message with the given type and payload.
func NewMessage(msgType MessageType, payload any) (*Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		ID:        generateMessageID(),
		Type:      msgType,
		Payload:   data,
		Timestamp: time.Now().UnixMilli(),
	}, nil
}

// generateMessageID generates a unique message ID with msg_ prefix.
func generateMessageID() string {
	return "msg_" + shortuuid.New()
}

// WelcomePayload is the payload for the welcome message sent by server after connection.
// Contains worknode info (name, labels) managed via Web UI.
type WelcomePayload struct {
	WorknodeID string   `json:"worknode_id"`
	Name       string   `json:"name"`
	Labels     []string `json:"labels"`
}

// HeartbeatPayload is the payload for heartbeat messages.
type HeartbeatPayload struct {
	WorknodeID  string            `json:"worknode_id"`
	Name        string            `json:"name"`
	Labels      []string          `json:"labels"`
	Version     string            `json:"version"`
	Environment *EnvironmentInfo  `json:"environment,omitempty"`
	Metrics     *HeartbeatMetrics `json:"metrics,omitempty"`
}

// EnvironmentInfo contains detailed environment information for LLM context.
// This is similar to Cursor's system reminder format.
type EnvironmentInfo struct {
	OS            string `json:"os"`              // e.g., "darwin", "linux", "windows"
	OSVersion     string `json:"os_version"`      // e.g., "24.6.0" for macOS
	Arch          string `json:"arch"`            // e.g., "amd64", "arm64"
	Hostname      string `json:"hostname"`        // Machine hostname
	Shell         string `json:"shell"`           // Default shell, e.g., "/bin/zsh"
	HomeDir       string `json:"home_dir"`        // User home directory
	WorkspaceRoot string `json:"workspace_root"`  // Configured workspace root
	Username      string `json:"username"`        // Current user
	NumCPU        int    `json:"num_cpu"`         // Number of CPUs
	TotalMemoryMB int64  `json:"total_memory_mb"` // Total memory in MB
	CurrentTime   string `json:"current_time"`    // Current time in RFC3339 format
	Timezone      string `json:"timezone"`        // Timezone name, e.g., "Asia/Shanghai"
	UTCOffset     string `json:"utc_offset"`      // UTC offset, e.g., "+08:00"
}

// HeartbeatMetrics contains system metrics.
type HeartbeatMetrics struct {
	CPUPercent    float64 `json:"cpu_percent,omitempty"`
	MemoryPercent float64 `json:"memory_percent,omitempty"`
	DiskPercent   float64 `json:"disk_percent,omitempty"`
}

// TaskOperation defines the type of workspace operation.
type TaskOperation string

const (
	TaskOpRead         TaskOperation = "read"
	TaskOpWrite        TaskOperation = "write"
	TaskOpList         TaskOperation = "list"
	TaskOpGlob         TaskOperation = "glob"
	TaskOpGrep         TaskOperation = "grep"
	TaskOpBash         TaskOperation = "bash"
	TaskOpWebFetch     TaskOperation = "webfetch"
	TaskOpMCPCall      TaskOperation = "mcp_call"
	TaskOpMCPListTools TaskOperation = "mcp_list_tools"
	TaskOpSyncSkill    TaskOperation = "sync_skill"
)

// TaskRequestPayload is the payload for task request messages.
type TaskRequestPayload struct {
	TaskID           string          `json:"task_id"`
	TraceID          string          `json:"trace_id,omitempty"`
	SourceInstanceID string          `json:"source_instance_id,omitempty"` // Safari instance ID
	Operation        TaskOperation   `json:"operation"`
	Args             json.RawMessage `json:"args"`
}

// ReadArgs are the arguments for read operation.
type ReadArgs struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset,omitempty"`
	Limit  int64  `json:"limit,omitempty"`
}

// WriteArgs are the arguments for write operation.
type WriteArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"` // base64 encoded
}

// ListArgs are the arguments for list operation.
type ListArgs struct {
	Path      string   `json:"path"`
	Recursive bool     `json:"recursive,omitempty"`
	Ignore    []string `json:"ignore,omitempty"`
}

// GlobArgs are the arguments for glob operation.
type GlobArgs struct {
	Pattern string `json:"pattern"`
}

// GrepArgs are the arguments for grep operation.
type GrepArgs struct {
	Pattern string   `json:"pattern"`
	Include []string `json:"include,omitempty"`
}

// BashArgs are the arguments for bash operation.
type BashArgs struct {
	Command string `json:"command"`
	Workdir string `json:"workdir,omitempty"`
	Timeout int    `json:"timeout,omitempty"` // seconds
}

// TaskOutputPayload is the payload for streaming task output.
type TaskOutputPayload struct {
	TaskID string `json:"task_id"`
	Stream string `json:"stream"` // stdout, stderr
	Data   string `json:"data"`
}

// TaskResultPayload is the payload for task completion.
type TaskResultPayload struct {
	TaskID           string          `json:"task_id"`
	SourceInstanceID string          `json:"source_instance_id,omitempty"` // Safari instance ID
	Success          bool            `json:"success"`
	Result           json.RawMessage `json:"result,omitempty"`
	Error            string          `json:"error,omitempty"`
	ExitCode         int             `json:"exit_code,omitempty"`
}

// ReadResult is the result of a read operation.
type ReadResult struct {
	Content   string `json:"content"` // base64 encoded
	TotalSize int64  `json:"total_size"`
}

// ListEntry is a single entry in a list result.
type ListEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// ListResult is the result of a list operation.
type ListResult struct {
	Entries []ListEntry `json:"entries"`
}

// GlobResult is the result of a glob operation.
type GlobResult struct {
	Matches []string `json:"matches"`
}

// GrepMatch is a single match in a grep result.
type GrepMatch struct {
	Path       string `json:"path"`
	LineNumber int    `json:"line_number"`
	Content    string `json:"content"`
}

// GrepResult is the result of a grep operation.
type GrepResult struct {
	Matches   []GrepMatch `json:"matches"`
	Content   string      `json:"content"`              // Formatted content (may be truncated)
	Truncated bool        `json:"truncated,omitempty"`  // Whether content was truncated
	FilePath  string      `json:"file_path,omitempty"`  // Path to full content if truncated
	TotalSize int64       `json:"total_size,omitempty"` // Original content size
}

// BashResult is the result of a bash operation.
type BashResult struct {
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	Truncated bool   `json:"truncated,omitempty"`  // Whether content was truncated
	FilePath  string `json:"file_path,omitempty"`  // Path to full content if truncated
	TotalSize int64  `json:"total_size,omitempty"` // Original content size
}

// WebFetchArgs are the arguments for webfetch operation.
type WebFetchArgs struct {
	URL     string `json:"url"`
	Format  string `json:"format,omitempty"`  // markdown, text, html (default: markdown)
	Timeout int    `json:"timeout,omitempty"` // seconds (default: 30, max: 120)
}

// WebFetchResult is the result of a webfetch operation.
type WebFetchResult struct {
	Content   string `json:"content"`
	URL       string `json:"url"`                  // Final URL after redirects
	Truncated bool   `json:"truncated,omitempty"`  // Whether content was truncated
	FilePath  string `json:"file_path,omitempty"`  // Path to full content if truncated
	TotalSize int64  `json:"total_size,omitempty"` // Original content size
}

// MCPCallArgs are the arguments for mcp_call operation.
type MCPCallArgs struct {
	Server   MCPServerConfig `json:"server"`
	ToolName string          `json:"tool_name"`
	Args     json.RawMessage `json:"args"`
	Timeout  int             `json:"timeout,omitempty"` // seconds
}

// MCPCallResult is the result of an mcp_call operation.
type MCPCallResult struct {
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`   // Whether the tool returned an error
	Truncated bool   `json:"truncated,omitempty"`  // Whether content was truncated
	FilePath  string `json:"file_path,omitempty"`  // Path to full content if truncated
	TotalSize int64  `json:"total_size,omitempty"` // Original content size
}

// MCPListToolsArgs are the arguments for mcp_list_tools operation.
type MCPListToolsArgs struct {
	Server MCPServerConfig `json:"server"`
}

// MCPListToolsResult is the result of an mcp_list_tools operation.
type MCPListToolsResult struct {
	Tools []MCPToolInfo `json:"tools"`
}

// SyncSkillArgs are the arguments for sync_skill operation.
type SyncSkillArgs struct {
	SkillName string `json:"skill_name"`
	SkillDir  string `json:"skill_dir"`
	ZipData   string `json:"zip_data"` // base64 encoded zip
	Checksum  string `json:"checksum"`
}

// SyncSkillResult is the result of a sync_skill operation.
type SyncSkillResult struct {
	Success bool   `json:"success"`
	Path    string `json:"path"` // Local path where skill was extracted
}

// MCPToolInfo represents metadata for an MCP tool.
type MCPToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema,omitempty"`
}

// TaskCancelPayload is the payload for task cancellation.
type TaskCancelPayload struct {
	TaskID string `json:"task_id"`
}

// MCPServerConfig is the MCP server configuration passed from cloud.
type MCPServerConfig struct {
	Name           string            `json:"name"`
	Transport      string            `json:"transport"` // stdio, sse
	Command        string            `json:"command,omitempty"`
	Args           []string          `json:"args,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	URL            string            `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	DynamicHeaders map[string]string `json:"dynamic_headers,omitempty"`
}

// MCPCallPayload is the payload for MCP tool calls.
// Server configuration is passed from cloud (stored in database).
type MCPCallPayload struct {
	CallID    string          `json:"call_id"`
	Server    MCPServerConfig `json:"server"`
	ToolName  string          `json:"tool_name"`
	Arguments json.RawMessage `json:"arguments"`
}

// MCPResultPayload is the payload for MCP call results.
type MCPResultPayload struct {
	CallID  string          `json:"call_id"`
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}
