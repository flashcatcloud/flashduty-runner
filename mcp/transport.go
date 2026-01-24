package mcp

import (
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"

	sdk_mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewStdioTransport creates a new stdio transport for MCP.
func NewStdioTransport(command string, args []string, env map[string]string) sdk_mcp.Transport {
	cmd := exec.Command(command, args...)
	cmd.Env = buildEnv(env)
	return &sdk_mcp.CommandTransport{
		Command: cmd,
	}
}

// buildEnv creates environment variables by inheriting current env and adding custom ones.
func buildEnv(customEnv map[string]string) []string {
	env := os.Environ()
	for k, v := range customEnv {
		if isValidEnvVar(k) {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// isValidEnvVar checks if environment variable name is valid.
func isValidEnvVar(name string) bool {
	return !strings.Contains(name, "=") && !strings.Contains(name, "\x00")
}

// NewSSETransport creates a new SSE transport for MCP.
func NewSSETransport(endpoint string, headers map[string]string, dynamicHeaders map[string]string) sdk_mcp.Transport {
	slog.Info("mcp creating SSE transport",
		"endpoint", endpoint,
		"headers_count", len(headers),
		"dynamic_headers_count", len(dynamicHeaders),
	)

	return &sdk_mcp.StreamableClientTransport{
		Endpoint: endpoint,
		HTTPClient: &http.Client{
			Transport: &headerTransport{
				headers:        headers,
				dynamicHeaders: dynamicHeaders,
				base:           http.DefaultTransport,
			},
		},
	}
}

// headerTransport adds custom headers to HTTP requests.
type headerTransport struct {
	headers        map[string]string
	dynamicHeaders map[string]string
	base           http.RoundTripper
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	for k, v := range t.dynamicHeaders {
		req.Header.Set(k, v)
	}

	slog.Debug("mcp http request",
		"method", req.Method,
		"url", req.URL.String(),
		"host", req.Host,
	)

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		slog.Error("mcp http request failed",
			"method", req.Method,
			"url", req.URL.String(),
			"error", err,
		)
		return nil, err
	}

	slog.Debug("mcp http response",
		"method", req.Method,
		"url", req.URL.String(),
		"status", resp.StatusCode,
	)

	return resp, nil
}
