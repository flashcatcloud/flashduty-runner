# Contributing to Flashduty Runner

Thank you for your interest in contributing to Flashduty Runner! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md). We aim to maintain a welcoming environment for all contributors.

## Getting Started

### Prerequisites

- Go 1.24 or later
- Make
- golangci-lint (for linting)
- gofumpt (for formatting)
- gci (for import sorting)

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

    "github.com/flashcatcloud/flashduty-runner/config"
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
go test -v ./workspace/...

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
│   └── main.go              # CLI entry point (cobra)
├── config/
│   ├── config.go            # Configuration loading (viper)
│   └── config_test.go       # Config tests
├── permission/
│   ├── permission.go        # Command permission checker
│   └── permission_test.go   # Permission tests
├── protocol/
│   └── messages.go          # WebSocket message types
├── workspace/
│   ├── workspace.go         # Workspace operations
│   ├── workspace_test.go    # Workspace tests
│   ├── webfetch.go          # Web page fetching
│   └── large_output.go      # Large output handling
├── ws/
│   ├── client.go            # WebSocket client
│   └── handler.go           # Message handler
├── mcp/
│   ├── client.go            # MCP client manager
│   └── transport.go         # MCP transport layer
├── .github/
│   ├── workflows/           # CI/CD pipelines
│   │   ├── go.yml           # Go tests
│   │   ├── lint.yml         # Linting
│   │   ├── goreleaser.yml   # Release automation
│   │   └── docker-publish.yml # Docker builds
│   ├── ISSUE_TEMPLATE/      # Issue templates
│   └── pull_request_template.md
├── Dockerfile               # Multi-stage Docker build
├── Makefile                 # Build automation
├── .goreleaser.yaml         # Release configuration
├── .golangci.yml            # Linter configuration
└── README.md
```

## Adding New Features

### New Workspace Operation

1. Add method to `workspace/workspace.go`
2. Add message type to `protocol/messages.go`
3. Add handler case in `ws/handler.go`
4. Add tests in `workspace/workspace_test.go`
5. Update README if user-facing

### New CLI Command

1. Add command function in `cmd/main.go`
2. Register with `rootCmd.AddCommand()`
3. Add tests
4. Update README

### New Configuration Option

1. Add field to `config.Config` struct in `config/config.go`
2. Update `DefaultConfig()` if needed
3. Add validation in `Validate()` if needed
4. Document in README
5. Add tests in `config/config_test.go`

## CI/CD

### Automated Checks

Every PR triggers:
- **go.yml**: Runs `go test` with race detection
- **lint.yml**: Runs golangci-lint
- **code-scanning.yml**: Security scanning with CodeQL

### Release Process

Releases are automated via GoReleaser when a tag is pushed:

```bash
git tag v1.0.0
git push origin v1.0.0
```

This triggers:
1. Cross-platform binary builds (Linux, macOS, Windows × amd64, arm64)
2. Docker image builds and push to GHCR
3. GitHub release creation with changelog

## Questions?

- Open an [issue](https://github.com/flashcatcloud/flashduty-runner/issues) for bugs or feature requests
- Check existing issues before creating new ones
- Join our community discussions

Thank you for contributing!
