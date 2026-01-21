# Contributing to Flashduty Runner

Thank you for your interest in contributing to Flashduty Runner! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions. We aim to maintain a welcoming environment for all contributors.

## Getting Started

### Prerequisites

- Go 1.22 or later
- Make
- golangci-lint (for linting)
- gofumpt (for formatting)

### Setup

```bash
# Clone the repository
git clone https://github.com/flashcatcloud/flashduty-runner.git
cd flashduty-runner

# Install development tools
make tools

# Install dependencies
go mod tidy

# Verify setup
make test
```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

### 2. Make Changes

Follow these coding standards:

- **Formatting**: Run `make fmt` before committing
- **Linting**: Ensure `make lint` passes
- **Testing**: Add tests for new functionality
- **Documentation**: Update README if adding features

### 3. Code Style

#### Imports

Use `gci` to organize imports in this order:
1. Standard library
2. Third-party packages
3. Local packages (github.com/flashcatcloud/...)

```go
import (
    "context"
    "fmt"

    "github.com/gorilla/websocket"

    "github.com/flashcatcloud/flashduty-runner/internal/config"
)
```

#### Error Handling

- Always wrap errors with context
- Use `fmt.Errorf("context: %w", err)` for wrapping
- Never ignore errors silently

```go
if err != nil {
    return fmt.Errorf("failed to connect: %w", err)
}
```

#### Logging

- Use structured logging with `log/slog`
- Include relevant context in log messages
- Use appropriate log levels (debug, info, warn, error)

```go
slog.Info("connected to server",
    "url", config.APIURL,
    "runner_id", runnerID,
)
```

### 4. Testing

```bash
# Run all tests
make test

# Run specific package tests
go test -v ./internal/workspace/...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 5. Commit Messages

Follow conventional commits:

```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Adding tests
- `chore`: Maintenance tasks

Examples:
```
feat(workspace): add glob pattern support for file search
fix(ws): handle reconnection on network timeout
docs(readme): add troubleshooting section
```

### 6. Pull Request

1. Push your branch
2. Create a Pull Request with:
   - Clear description of changes
   - Link to related issues
   - Screenshots for UI changes (if any)
3. Wait for CI to pass
4. Address review comments

## Project Structure

```
flashduty-runner/
├── cmd/
│   └── flashduty-runner/
│       └── main.go              # Entry point
├── internal/
│   ├── config/                  # Configuration loading
│   ├── auth/                    # API Key authentication
│   ├── ws/                      # WebSocket client
│   ├── workspace/               # Workspace operations
│   ├── permission/              # Command permission check
│   ├── mcp/                     # MCP client manager
│   └── update/                  # Self-update logic
├── pkg/
│   └── protocol/                # WebSocket message protocol
├── .github/
│   └── workflows/               # CI/CD pipelines
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Adding New Features

### New Workspace Operation

1. Add method to `internal/workspace/workspace.go`
2. Add message type to `pkg/protocol/messages.go`
3. Add handler in `internal/ws/handler.go`
4. Add tests in `internal/workspace/workspace_test.go`
5. Update README if user-facing

### New CLI Command

1. Add command file in `cmd/flashduty-runner/`
2. Register in `cmd/flashduty-runner/root.go`
3. Add tests
4. Update README

## Questions?

- Open an issue for bugs or feature requests
- Check existing issues before creating new ones
- Join our community discussions

Thank you for contributing!
