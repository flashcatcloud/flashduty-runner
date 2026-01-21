// Package main is the entry point for flashduty-runner.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/flashcatcloud/flashduty-runner/config"
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

var configPath string

func main() {
	rootCmd := &cobra.Command{
		Use:   "flashduty-runner",
		Short: "Flashduty Runner - Execute commands on behalf of Flashduty",
		Long: `Flashduty Runner is a lightweight agent that runs in your environment
to execute commands and access resources on behalf of Flashduty platform.

It connects to Flashduty platform via WebSocket and executes workspace operations
(bash, read, write, list, glob, grep, webfetch) and MCP tool calls.`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path (default: ~/.flashduty-runner/config.yaml)")

	// Add subcommands
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(updateCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start the runner and connect to Flashduty",
		Long:  `Start the runner, connect to Flashduty cloud via WebSocket, and begin processing tasks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunner()
		},
	}
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

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check for updates and install if available",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Update check not implemented yet")
			return nil
		},
	}
}

func runRunner() error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logging
	setupLogging(cfg.Log)

	slog.Info("starting flashduty-runner",
		"version", Version,
		"name", cfg.Name,
		"labels", cfg.Labels,
	)

	// Create permission checker
	checker := permission.NewChecker(cfg.Permission.Bash)

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
	client := ws.NewClient(cfg, handler.Handle, Version)
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

func setupLogging(logCfg config.LogConfig) {
	level := parseLogLevel(logCfg.Level)
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if logCfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

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
