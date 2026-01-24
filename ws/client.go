// Package ws implements the WebSocket client for Flashduty communication.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/flashcatcloud/flashduty-runner/protocol"
)

const (
	// Heartbeat interval
	heartbeatInterval = 30 * time.Second

	// Write deadline
	writeWait = 10 * time.Second

	// Pong wait time
	pongWait = 60 * time.Second

	// Maximum reconnect attempts
	maxReconnectAttempts = 10

	// Initial reconnect delay
	initialReconnectDelay = 1 * time.Second

	// Maximum reconnect delay
	maxReconnectDelay = 5 * time.Minute
)

// MessageHandler handles incoming messages from Flashduty.
type MessageHandler func(ctx context.Context, msg *protocol.Message) error

// Client is the WebSocket client for Flashduty communication.
type Client struct {
	token         string
	apiURL        string
	workspaceRoot string
	handler       MessageHandler
	version       string
	envInfo       *protocol.EnvironmentInfo

	mu          sync.Mutex
	conn        *websocket.Conn
	closed      bool
	envInfoSent bool // Track if environment info has been sent
	stopCh      chan struct{}
	doneCh      chan struct{}
	sendCh      chan *protocol.Message

	// Worknode info from welcome message
	worknodeID string
	name       string
	labels     []string
}

// NewClient creates a new WebSocket client.
func NewClient(token, apiURL, workspaceRoot string, handler MessageHandler, version string) *Client {
	return &Client{
		token:         token,
		apiURL:        apiURL,
		workspaceRoot: workspaceRoot,
		handler:       handler,
		version:       version,
		envInfo:       collectEnvironmentInfo(workspaceRoot),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
		sendCh:        make(chan *protocol.Message, 100),
	}
}

// Connect establishes a WebSocket connection to Flashduty.
func (c *Client) Connect(ctx context.Context) error {
	// Build URL with token as query parameter
	u, err := url.Parse(c.apiURL)
	if err != nil {
		return fmt.Errorf("invalid API URL: %w", err)
	}

	q := u.Query()
	q.Set("token", c.token)
	u.RawQuery = q.Encode()

	slog.Info("connecting to Flashduty",
		"url", c.apiURL,
	)

	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		if resp != nil {
			_ = resp.Body.Close()
			return fmt.Errorf("failed to connect: %w (status: %d)", err, resp.StatusCode)
		}
		return fmt.Errorf("failed to connect: %w", err)
	}

	if resp != nil {
		_ = resp.Body.Close()
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	// Read welcome message to get worknode info (name, labels)
	if err := c.readWelcomeMessage(); err != nil {
		slog.Warn("failed to read welcome message", "error", err)
	}

	slog.Info("connected to Flashduty",
		"worknode_id", c.worknodeID,
		"name", c.name,
		"labels", c.labels,
	)

	return nil
}

// readWelcomeMessage reads the initial welcome message containing worknode info.
func (c *Client) readWelcomeMessage() error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	// Set a short deadline for the welcome message
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer func() {
		_ = conn.SetReadDeadline(time.Time{})
	}()

	_, data, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read welcome: %w", err)
	}

	var msg protocol.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	if msg.Type != protocol.MessageTypeWelcome {
		return fmt.Errorf("expected welcome message, got %s", msg.Type)
	}

	var welcome protocol.WelcomePayload
	if err := json.Unmarshal(msg.Payload, &welcome); err != nil {
		return fmt.Errorf("failed to parse welcome payload: %w", err)
	}

	c.worknodeID = welcome.WorknodeID
	c.name = welcome.Name
	c.labels = welcome.Labels

	return nil
}

// Run starts the client's read/write loops.
func (c *Client) Run(ctx context.Context) error {
	defer close(c.doneCh)

	// Start heartbeat
	go c.heartbeatLoop(ctx)

	// Start send loop
	go c.sendLoop(ctx)

	// Read loop (blocking)
	return c.readLoop(ctx)
}

// RunWithReconnect runs the client with automatic reconnection.
func (c *Client) RunWithReconnect(ctx context.Context) error {
	attempt := 0
	delay := initialReconnectDelay

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopCh:
			return nil
		default:
		}

		// Connect
		if err := c.Connect(ctx); err != nil {
			attempt++
			if attempt > maxReconnectAttempts {
				return fmt.Errorf("max reconnect attempts exceeded: %w", err)
			}

			slog.Warn("connection failed, retrying",
				"attempt", attempt,
				"delay", delay,
				"error", err,
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			// Exponential backoff
			delay *= 2
			if delay > maxReconnectDelay {
				delay = maxReconnectDelay
			}
			continue
		}

		// Reset on successful connection
		attempt = 0
		delay = initialReconnectDelay

		// Run (blocking until disconnect)
		if err := c.Run(ctx); err != nil {
			slog.Warn("connection lost",
				"error", err,
			)
		}

		// Check if intentionally closed
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return nil
		}

		// Reset state for reconnect
		c.mu.Lock()
		c.envInfoSent = false // Re-send environment info after reconnect
		c.mu.Unlock()
		c.doneCh = make(chan struct{})
		c.sendCh = make(chan *protocol.Message, 100)
	}
}

// Send sends a message to Flashduty.
func (c *Client) Send(msg *protocol.Message) error {
	select {
	case c.sendCh <- msg:
		return nil
	default:
		return fmt.Errorf("send channel full")
	}
}

// SendPayload sends a message with the given type and payload.
func (c *Client) SendPayload(msgType protocol.MessageType, payload any) error {
	msg, err := protocol.NewMessage(msgType, payload)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}
	return c.Send(msg)
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	close(c.stopCh)

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// WorknodeID returns the worknode ID assigned by Flashduty.
func (c *Client) WorknodeID() string {
	return c.worknodeID
}

// Name returns the worknode name from welcome message.
func (c *Client) Name() string {
	return c.name
}

// Labels returns the worknode labels from welcome message.
func (c *Client) Labels() []string {
	return c.labels
}

func (c *Client) readLoop(ctx context.Context) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	// Handle ping messages from server - must reply with pong and reset deadline
	conn.SetPingHandler(func(appData string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		// Must reply with pong - WriteControl is safe to call from handler
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeWait))
	})

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.stopCh:
			return nil
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		// Reset deadline on any message received
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Warn("failed to unmarshal message",
				"error", err,
			)
			continue
		}

		// Handle message in goroutine to not block read loop
		go func(m protocol.Message) {
			if err := c.handler(ctx, &m); err != nil {
				slog.Error("failed to handle message",
					"type", m.Type,
					"error", err,
				)
			}
		}(msg)
	}
}

func (c *Client) sendLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-c.doneCh:
			return
		case msg := <-c.sendCh:
			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()

			if conn == nil {
				continue
			}

			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			data, err := json.Marshal(msg)
			if err != nil {
				slog.Error("failed to marshal message",
					"error", err,
				)
				continue
			}

			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				slog.Error("failed to send message",
					"error", err,
				)
			}
		}
	}
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// Send initial heartbeat
	c.sendHeartbeat()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-c.doneCh:
			return
		case <-ticker.C:
			c.sendHeartbeat()
		}
	}
}

func (c *Client) sendHeartbeat() {
	payload := protocol.HeartbeatPayload{
		WorknodeID: c.worknodeID,
		Name:       c.name,
		Labels:     c.labels,
		Version:    c.version,
	}

	// Only send environment info on first heartbeat after connection
	// Environment info is static and doesn't need to be sent repeatedly
	c.mu.Lock()
	if !c.envInfoSent {
		payload.Environment = c.envInfo
		c.envInfoSent = true
		slog.Debug("sending environment info with first heartbeat")
	}
	c.mu.Unlock()

	if err := c.SendPayload(protocol.MessageTypeHeartbeat, payload); err != nil {
		slog.Warn("failed to send heartbeat",
			"error", err,
		)
	}
}

// collectEnvironmentInfo gathers system environment information.
func collectEnvironmentInfo(workspaceRoot string) *protocol.EnvironmentInfo {
	info := &protocol.EnvironmentInfo{
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		NumCPU:        runtime.NumCPU(),
		WorkspaceRoot: workspaceRoot,
		OSVersion:     getOSVersion(),
		Shell:         getDefaultShell(),
		TotalMemoryMB: getTotalMemoryMB(),
	}

	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	if u, err := user.Current(); err == nil {
		info.Username = u.Username
		info.HomeDir = u.HomeDir
	}

	now := time.Now()
	info.CurrentTime = now.Format(time.RFC3339)
	info.Timezone = now.Location().String()
	info.UTCOffset = now.Format("-07:00")

	return info
}

// getOSVersion returns the OS version string.
func getOSVersion() string {
	switch runtime.GOOS {
	case "darwin":
		return getCommandOutput("sw_vers", "-productVersion") // Try macOS version first
	case "linux":
		if version := getLinuxVersion(); version != "" {
			return version
		}
	case "windows":
		return getCommandOutput("cmd", "/c", "ver")
	}
	return getCommandOutput("uname", "-r") // Fallback
}

// getLinuxVersion tries to get Linux version from /etc/os-release.
func getLinuxVersion() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VERSION_ID=") {
			return strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}
	return ""
}

// getCommandOutput executes a command and returns its trimmed output.
func getCommandOutput(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// getDefaultShell returns the default shell path.
func getDefaultShell() string {
	// Check SHELL environment variable first
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}

	// Platform-specific defaults
	switch runtime.GOOS {
	case "windows":
		if comspec := os.Getenv("COMSPEC"); comspec != "" {
			return comspec
		}
		return "cmd.exe"
	default:
		return "/bin/sh"
	}
}

// getTotalMemoryMB returns total system memory in MB.
func getTotalMemoryMB() int64 {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			var bytes int64
			if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &bytes); err == nil {
				return bytes / (1024 * 1024)
			}
		}
	case "linux":
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "MemTotal:") {
					var kb int64
					if _, err := fmt.Sscanf(line, "MemTotal: %d kB", &kb); err == nil {
						return kb / 1024
					}
				}
			}
		}
	}
	return 0
}
