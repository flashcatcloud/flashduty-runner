// Package mcp implements MCP client for executing tool calls on behalf of cloud.
package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/flashcatcloud/flashduty-runner/protocol"

	sdk_mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Re-export SDK types for use by other packages.
type (
	CallToolResult = sdk_mcp.CallToolResult
	TextContent    = sdk_mcp.TextContent
	ImageContent   = sdk_mcp.ImageContent
)

const (
	DefaultConnectTimeout = 30 * time.Second
	DefaultCallTimeout    = 60 * time.Second
)

// ClientManager manages MCP server connections and sessions.
type ClientManager struct {
	mu       sync.Mutex
	clients  map[string]*sdk_mcp.Client
	sessions map[string]*sdk_mcp.ClientSession
}

// NewClientManager creates a new ClientManager.
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:  make(map[string]*sdk_mcp.Client),
		sessions: make(map[string]*sdk_mcp.ClientSession),
	}
}

// GetSession returns or creates an MCP session for the given server.
func (m *ClientManager) GetSession(ctx context.Context, server *protocol.MCPServerConfig) (*sdk_mcp.ClientSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	serverName := server.Name

	slog.Debug("mcp get session",
		"server_name", serverName,
		"transport", server.Transport,
		"url", server.URL,
		"command", server.Command,
		"has_headers", len(server.Headers) > 0,
		"has_dynamic_headers", len(server.DynamicHeaders) > 0,
	)

	// Check if session exists and is still valid
	if session, ok := m.sessions[serverName]; ok {
		slog.Debug("mcp reusing existing session", "server_name", serverName)
		return session, nil
	}

	// Create client if not exists
	client, ok := m.clients[serverName]
	if !ok {
		client = sdk_mcp.NewClient(&sdk_mcp.Implementation{
			Name:    "flashduty-runner",
			Version: "1.0.0",
		}, nil)
		m.clients[serverName] = client
	}

	// Create transport
	slog.Info("mcp creating transport",
		"server_name", serverName,
		"transport", server.Transport,
		"url", server.URL,
		"command", server.Command,
	)

	transport, err := createTransport(server)
	if err != nil {
		slog.Error("mcp create transport failed", "server_name", serverName, "error", err)
		return nil, err
	}

	// Connect with timeout
	slog.Info("mcp connecting to server", "server_name", serverName, "timeout", DefaultConnectTimeout)

	connectCtx, cancel := context.WithTimeout(ctx, DefaultConnectTimeout)
	defer cancel()

	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		slog.Error("mcp connect failed",
			"server_name", serverName,
			"transport", server.Transport,
			"url", server.URL,
			"error", err,
		)
		return nil, fmt.Errorf("failed to connect to MCP server '%s': %w", serverName, err)
	}

	slog.Info("mcp connected successfully", "server_name", serverName)
	m.sessions[serverName] = session
	return session, nil
}

// CallTool executes a single MCP tool call using the manager for session persistence.
func (m *ClientManager) CallTool(ctx context.Context, server *protocol.MCPServerConfig, toolName string, arguments map[string]any) (*sdk_mcp.CallToolResult, error) {
	slog.Info("mcp call tool",
		"server_name", server.Name,
		"tool_name", toolName,
		"arguments", arguments,
	)

	session, err := m.GetSession(ctx, server)
	if err != nil {
		slog.Error("mcp get session failed", "server_name", server.Name, "error", err)
		return nil, err
	}

	callCtx, callCancel := context.WithTimeout(ctx, DefaultCallTimeout)
	defer callCancel()

	slog.Debug("mcp calling tool", "server_name", server.Name, "tool_name", toolName, "timeout", DefaultCallTimeout)

	result, err := session.CallTool(callCtx, &sdk_mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		slog.Error("mcp call tool failed",
			"server_name", server.Name,
			"tool_name", toolName,
			"error", err,
		)
		m.invalidateSession(server.Name)
		return nil, fmt.Errorf("failed to call tool '%s': %w", toolName, err)
	}

	slog.Info("mcp call tool success",
		"server_name", server.Name,
		"tool_name", toolName,
		"is_error", result.IsError,
	)

	return result, nil
}

// ListTools lists available tools from an MCP server using the manager for session persistence.
func (m *ClientManager) ListTools(ctx context.Context, server *protocol.MCPServerConfig) ([]*sdk_mcp.Tool, error) {
	session, err := m.GetSession(ctx, server)
	if err != nil {
		return nil, err
	}

	var allTools []*sdk_mcp.Tool
	var cursor string
	for {
		result, err := session.ListTools(ctx, &sdk_mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			m.invalidateSession(server.Name)
			return nil, fmt.Errorf("failed to list tools: %w", err)
		}
		allTools = append(allTools, result.Tools...)
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	return allTools, nil
}

// invalidateSession removes a session from cache to force reconnection.
func (m *ClientManager) invalidateSession(serverName string) {
	m.mu.Lock()
	delete(m.sessions, serverName)
	m.mu.Unlock()
}

// Close closes all active sessions and clients.
func (m *ClientManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, session := range m.sessions {
		_ = session.Close()
		delete(m.sessions, name)
	}
	m.clients = make(map[string]*sdk_mcp.Client)
}

var (
	defaultManager     *ClientManager
	defaultManagerOnce sync.Once
)

// GetDefaultManager returns a singleton ClientManager.
func GetDefaultManager() *ClientManager {
	defaultManagerOnce.Do(func() {
		defaultManager = NewClientManager()
	})
	return defaultManager
}

// CallTool executes a single MCP tool call using the default manager.
func CallTool(ctx context.Context, server *protocol.MCPServerConfig, toolName string, arguments map[string]any) (*sdk_mcp.CallToolResult, error) {
	return GetDefaultManager().CallTool(ctx, server, toolName, arguments)
}

// ListTools lists available tools from an MCP server using the default manager.
func ListTools(ctx context.Context, server *protocol.MCPServerConfig) ([]*sdk_mcp.Tool, error) {
	return GetDefaultManager().ListTools(ctx, server)
}

// createTransport creates an MCP transport based on server configuration.
func createTransport(server *protocol.MCPServerConfig) (sdk_mcp.Transport, error) {
	switch server.Transport {
	case "stdio":
		return NewStdioTransport(server.Command, server.Args, server.Env), nil
	case "sse":
		return NewSSETransport(server.URL, server.Headers, server.DynamicHeaders), nil
	default:
		return nil, fmt.Errorf("unsupported transport type '%s'", server.Transport)
	}
}

// ExtractContent extracts text content from MCP CallToolResult.
func ExtractContent(result *sdk_mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}

	var parts []string
	for _, c := range result.Content {
		switch v := c.(type) {
		case *sdk_mcp.TextContent:
			parts = append(parts, v.Text)
		case *sdk_mcp.ImageContent:
			parts = append(parts, fmt.Sprintf("[Image: %s]", v.MIMEType))
		}
	}
	return strings.Join(parts, "\n")
}
