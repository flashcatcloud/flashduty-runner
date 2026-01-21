package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/flashcatcloud/flashduty-runner/protocol"
	"github.com/flashcatcloud/flashduty-runner/workspace"
)

// Handler handles incoming WebSocket messages.
type Handler struct {
	ws     *workspace.Workspace
	client *Client

	// Track running tasks for cancellation and graceful shutdown
	mu          sync.RWMutex
	runningTask map[string]context.CancelFunc
	taskWg      sync.WaitGroup // For graceful shutdown
}

// NewHandler creates a new message handler.
func NewHandler(ws *workspace.Workspace) *Handler {
	return &Handler{
		ws:          ws,
		runningTask: make(map[string]context.CancelFunc),
	}
}

// SetClient sets the WebSocket client for sending responses.
func (h *Handler) SetClient(client *Client) {
	h.client = client
}

// WaitForTasks waits for all running tasks to complete with a timeout.
// Returns true if all tasks completed, false if timeout occurred.
func (h *Handler) WaitForTasks(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		h.taskWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// CancelAllTasks cancels all running tasks.
func (h *Handler) CancelAllTasks() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for taskID, cancel := range h.runningTask {
		slog.Info("cancelling task due to shutdown", "task_id", taskID)
		cancel()
	}
}

// RunningTaskCount returns the number of running tasks.
func (h *Handler) RunningTaskCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.runningTask)
}

// Handle handles an incoming message.
func (h *Handler) Handle(ctx context.Context, msg *protocol.Message) error {
	switch msg.Type {
	case protocol.MessageTypeTaskRequest:
		return h.handleTaskRequest(ctx, msg)
	case protocol.MessageTypeTaskCancel:
		return h.handleTaskCancel(ctx, msg)
	case protocol.MessageTypeMCPCall:
		return h.handleMCPCall(ctx, msg)
	default:
		slog.Warn("unknown message type",
			"type", msg.Type,
		)
		return nil
	}
}

func (h *Handler) handleTaskRequest(ctx context.Context, msg *protocol.Message) error {
	var req protocol.TaskRequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return fmt.Errorf("failed to unmarshal task request: %w", err)
	}

	slog.Info("received task request", "task_id", req.TaskID, "operation", req.Operation)

	taskCtx, cancel := context.WithCancel(ctx)
	h.mu.Lock()
	h.runningTask[req.TaskID] = cancel
	h.mu.Unlock()

	h.taskWg.Add(1)
	go func() {
		defer h.taskWg.Done()
		h.executeAndSendResult(taskCtx, &req)
	}()
	return nil
}

// unregisterTask removes a task from the running tasks map.
func (h *Handler) unregisterTask(taskID string) {
	h.mu.Lock()
	delete(h.runningTask, taskID)
	h.mu.Unlock()
}

// executeAndSendResult executes a task and sends the result.
func (h *Handler) executeAndSendResult(ctx context.Context, req *protocol.TaskRequestPayload) {
	defer h.unregisterTask(req.TaskID)

	result, err := h.executeTask(ctx, req)
	if err != nil {
		h.sendTaskResult(req.TaskID, req.SourceInstanceID, false, nil, err.Error(), 1)
		return
	}

	h.sendTaskResult(req.TaskID, req.SourceInstanceID, true, result, "", 0)
}

func parseArgs[T any](data json.RawMessage) (*T, error) {
	var args T
	if err := json.Unmarshal(data, &args); err != nil {
		return nil, err
	}
	return &args, nil
}

func (h *Handler) executeTask(ctx context.Context, req *protocol.TaskRequestPayload) (any, error) {
	switch req.Operation {
	case protocol.TaskOpRead:
		args, err := parseArgs[protocol.ReadArgs](req.Args)
		if err != nil {
			return nil, fmt.Errorf("invalid read args: %w", err)
		}
		return h.ws.Read(ctx, args)

	case protocol.TaskOpWrite:
		args, err := parseArgs[protocol.WriteArgs](req.Args)
		if err != nil {
			return nil, fmt.Errorf("invalid write args: %w", err)
		}
		if err := h.ws.Write(ctx, args); err != nil {
			return nil, err
		}
		return map[string]bool{"success": true}, nil

	case protocol.TaskOpList:
		args, err := parseArgs[protocol.ListArgs](req.Args)
		if err != nil {
			return nil, fmt.Errorf("invalid list args: %w", err)
		}
		return h.ws.List(ctx, args)

	case protocol.TaskOpGlob:
		args, err := parseArgs[protocol.GlobArgs](req.Args)
		if err != nil {
			return nil, fmt.Errorf("invalid glob args: %w", err)
		}
		return h.ws.Glob(ctx, args)

	case protocol.TaskOpGrep:
		args, err := parseArgs[protocol.GrepArgs](req.Args)
		if err != nil {
			return nil, fmt.Errorf("invalid grep args: %w", err)
		}
		return h.ws.Grep(ctx, args)

	case protocol.TaskOpBash:
		args, err := parseArgs[protocol.BashArgs](req.Args)
		if err != nil {
			return nil, fmt.Errorf("invalid bash args: %w", err)
		}
		return h.ws.Bash(ctx, args)

	case protocol.TaskOpWebFetch:
		args, err := parseArgs[protocol.WebFetchArgs](req.Args)
		if err != nil {
			return nil, fmt.Errorf("invalid webfetch args: %w", err)
		}
		return h.ws.WebFetch(ctx, args)

	case protocol.TaskOpMCPCall:
		args, err := parseArgs[protocol.MCPCallArgs](req.Args)
		if err != nil {
			return nil, fmt.Errorf("invalid mcp_call args: %w", err)
		}
		return h.ws.MCPCall(ctx, args)

	case protocol.TaskOpMCPListTools:
		args, err := parseArgs[protocol.MCPListToolsArgs](req.Args)
		if err != nil {
			return nil, fmt.Errorf("invalid mcp_list_tools args: %w", err)
		}
		return h.ws.MCPListTools(ctx, args)

	case protocol.TaskOpSyncSkill:
		args, err := parseArgs[protocol.SyncSkillArgs](req.Args)
		if err != nil {
			return nil, fmt.Errorf("invalid sync_skill args: %w", err)
		}
		return h.ws.SyncSkill(ctx, args)

	default:
		return nil, fmt.Errorf("unknown operation: %s", req.Operation)
	}
}

func (h *Handler) sendTaskResult(taskID, sourceInstanceID string, success bool, result any, errMsg string, exitCode int) {
	h.sendPayload(protocol.MessageTypeTaskResult, protocol.TaskResultPayload{
		TaskID:           taskID,
		SourceInstanceID: sourceInstanceID,
		Success:          success,
		Result:           marshalResult(result),
		Error:            errMsg,
		ExitCode:         exitCode,
	})
}

func (h *Handler) sendMCPResult(callID string, success bool, result any, errMsg string) {
	h.sendPayload(protocol.MessageTypeMCPResult, protocol.MCPResultPayload{
		CallID:  callID,
		Success: success,
		Result:  marshalResult(result),
		Error:   errMsg,
	})
}

func (h *Handler) sendPayload(msgType protocol.MessageType, payload any) {
	if h.client == nil {
		slog.Error("client not set, cannot send message", "type", msgType)
		return
	}
	if err := h.client.SendPayload(msgType, payload); err != nil {
		slog.Error("failed to send message", "type", msgType, "error", err)
	}
}

func (h *Handler) handleTaskCancel(ctx context.Context, msg *protocol.Message) error {
	var payload protocol.TaskCancelPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal task cancel: %w", err)
	}

	h.mu.RLock()
	cancel, ok := h.runningTask[payload.TaskID]
	h.mu.RUnlock()

	if ok {
		slog.Info("cancelling task", "task_id", payload.TaskID)
		cancel()
	}
	return nil
}

func (h *Handler) handleMCPCall(ctx context.Context, msg *protocol.Message) error {
	var payload protocol.MCPCallPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal mcp call: %w", err)
	}

	go func() {
		result, err := h.ws.MCPCall(ctx, &protocol.MCPCallArgs{
			Server:   payload.Server,
			ToolName: payload.ToolName,
			Args:     payload.Arguments,
		})
		if err != nil {
			h.sendMCPResult(payload.CallID, false, nil, err.Error())
		} else {
			h.sendMCPResult(payload.CallID, true, result, "")
		}
	}()

	return nil
}

// marshalResult marshals a result to JSON RawMessage, returns nil on error.
func marshalResult(result any) json.RawMessage {
	if result == nil {
		return nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		slog.Error("failed to marshal result", "error", err)
		return nil
	}
	return data
}
