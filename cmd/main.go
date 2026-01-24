// Package main is the entry point for flashduty-runner.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/flashcatcloud/flashduty-runner/permission"
	"github.com/flashcatcloud/flashduty-runner/workspace"
	"github.com/flashcatcloud/flashduty-runner/ws"
)

// Build-time variables
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// Command line flags
var (
	flagToken     string
	flagURL       string
	flagWorkspace string
	flagLogLevel  string
)

// Default values
const (
	defaultURL      = "wss://api.flashcat.cloud/safari/worknode/ws"
	defaultLogLevel = "info"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "flashduty-runner",
		Short: "Flashduty Runner - Execute commands on behalf of Flashduty",
		Long: `Flashduty Runner is a lightweight agent that runs in your environment
to execute commands and access resources on behalf of Flashduty platform.

It connects to Flashduty platform via WebSocket and executes workspace operations
(bash, read, write, list, glob, grep, webfetch) and MCP tool calls.`,
	}

	// Add subcommands
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the runner and connect to Flashduty",
		Long: `Start the runner, connect to Flashduty cloud via WebSocket, and begin processing tasks.

Examples:
  # Basic usage (token required)
  flashduty-runner run --token wnt_xxx

  # Specify workspace directory
  flashduty-runner run --token wnt_xxx --workspace ~/projects

  # Specify custom API URL
  flashduty-runner run --token wnt_xxx --url wss://custom.example.com/safari/worknode/ws

Environment variables:
  FLASHDUTY_RUNNER_TOKEN     - Authentication token (required if --token not provided)
  FLASHDUTY_RUNNER_URL       - WebSocket endpoint URL
  FLASHDUTY_RUNNER_WORKSPACE - Workspace root directory`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunner()
		},
	}

	// Flags with environment variable fallback
	cmd.Flags().StringVar(&flagToken, "token", "", "Authentication token (required, env: FLASHDUTY_RUNNER_TOKEN)")
	cmd.Flags().StringVar(&flagURL, "url", "", "WebSocket endpoint URL (env: FLASHDUTY_RUNNER_URL)")
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Workspace root directory (env: FLASHDUTY_RUNNER_WORKSPACE)")
	cmd.Flags().StringVar(&flagLogLevel, "log-level", "", "Log level: debug, info, warn, error (env: FLASHDUTY_RUNNER_LOG_LEVEL)")

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("flashduty-runner %s\n", Version)
			fmt.Printf("  Build Time: %s\n", BuildTime)
			fmt.Printf("  Git Commit: %s\n", GitCommit)
		},
	}
}

// Config holds the runtime configuration
type Config struct {
	Token         string
	URL           string
	WorkspaceRoot string
	LogLevel      string
}

func loadConfig() (*Config, error) {
	cfg := &Config{}

	// Token: flag > env
	cfg.Token = flagToken
	if cfg.Token == "" {
		cfg.Token = os.Getenv("FLASHDUTY_RUNNER_TOKEN")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("token is required: use --token flag or set FLASHDUTY_RUNNER_TOKEN environment variable")
	}

	// URL: flag > env > default
	cfg.URL = flagURL
	if cfg.URL == "" {
		cfg.URL = os.Getenv("FLASHDUTY_RUNNER_URL")
	}
	if cfg.URL == "" {
		cfg.URL = defaultURL
	}

	// Workspace: flag > env > default
	cfg.WorkspaceRoot = flagWorkspace
	if cfg.WorkspaceRoot == "" {
		cfg.WorkspaceRoot = os.Getenv("FLASHDUTY_RUNNER_WORKSPACE")
	}
	if cfg.WorkspaceRoot == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cfg.WorkspaceRoot = filepath.Join(homeDir, ".flashduty-runner", "workspace")
	}

	// Log level: flag > env > default
	cfg.LogLevel = flagLogLevel
	if cfg.LogLevel == "" {
		cfg.LogLevel = os.Getenv("FLASHDUTY_RUNNER_LOG_LEVEL")
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaultLogLevel
	}

	return cfg, nil
}

func runRunner() error {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Setup logging
	setupLogging(cfg.LogLevel)

	slog.Info("starting flashduty-runner",
		"version", Version,
		"workspace", cfg.WorkspaceRoot,
	)

	// Create permission checker with default deny-all policy
	checker := permission.NewChecker(map[string]string{"*": "deny"})

	// Create workspace
	wspace, err := workspace.New(cfg.WorkspaceRoot, checker)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	slog.Info("workspace initialized",
		"root", wspace.Root(),
	)

	// Create message handler
	handler := ws.NewHandler(wspace)

	// Create WebSocket client
	client := ws.NewClient(cfg.Token, cfg.URL, cfg.WorkspaceRoot, handler.Handle, Version)
	handler.SetClient(client)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, initiating graceful shutdown",
			"signal", sig,
		)

		// Cancel context to stop accepting new tasks
		cancel()

		// Wait for running tasks to complete (with timeout)
		taskCount := handler.RunningTaskCount()
		if taskCount > 0 {
			slog.Info("waiting for running tasks to complete",
				"count", taskCount,
			)
			if handler.WaitForTasks(30 * time.Second) {
				slog.Info("all tasks completed")
			} else {
				slog.Warn("task wait timeout, cancelling remaining tasks")
				handler.CancelAllTasks()
				// Give tasks a moment to clean up
				handler.WaitForTasks(5 * time.Second)
			}
		}

		_ = client.Close()
	}()

	// Run with reconnection
	if err := client.RunWithReconnect(ctx); err != nil {
		if ctx.Err() != nil {
			slog.Info("runner stopped gracefully")
			return nil
		}
		return fmt.Errorf("runner error: %w", err)
	}

	return nil
}

func setupLogging(levelStr string) {
	level := parseLogLevel(levelStr)
	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewTextHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(handler))
}

func parseLogLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
