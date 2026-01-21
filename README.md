# Flashduty Runner

English | [中文](README_zh.md)

Flashduty Runner is a lightweight agent that runs in your environment to execute commands and access resources on behalf of Flashduty AI SRE platform.

## Features

- **Secure Connection**: Connects to Flashduty cloud via WebSocket with API Key authentication
- **Workspace Operations**: Execute bash commands, read/write files, search with grep/glob
- **Permission Control**: Glob-based command whitelist/blacklist for security
- **Label-based Routing**: Tag runners for task routing (e.g., `k8s`, `production`)
- **Auto Update**: Automatic binary updates with version checking
- **MCP Proxy**: Connect to internal MCP servers through the runner

## Quick Start

### Installation

Download the latest release for your platform:

```bash
# Linux (amd64)
curl -LO https://github.com/flashcatcloud/flashduty-runner/releases/latest/download/flashduty-runner-linux-amd64
chmod +x flashduty-runner-linux-amd64
sudo mv flashduty-runner-linux-amd64 /usr/local/bin/flashduty-runner

# macOS (arm64)
curl -LO https://github.com/flashcatcloud/flashduty-runner/releases/latest/download/flashduty-runner-darwin-arm64
chmod +x flashduty-runner-darwin-arm64
sudo mv flashduty-runner-darwin-arm64 /usr/local/bin/flashduty-runner
```

### Configuration

Create a configuration file at `~/.flashduty-runner/config.yaml`:

```yaml
# API Key from Flashduty Console (required)
api_key: "fk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# Flashduty WebSocket endpoint
API_url: "wss://api.flashcat.cloud/runner/ws"

# Runner identification
name: "prod-k8s-runner"

# Labels for task routing
labels:
  - k8s
  - production
  - mysql

# Workspace root directory
workspace_root: "/var/flashduty/workspace"

# Auto update settings
auto_update: true

# Command permission (glob pattern matching)
permission:
  bash:
    "*": "deny"           # Deny by default
    "git *": "allow"
    "kubectl get *": "allow"
    "kubectl describe *": "allow"
    "kubectl logs *": "allow"
    "grep *": "allow"
    "cat *": "allow"
    "ls *": "allow"
```

### Running

```bash
# Start the runner
flashduty-runner run

# Start with custom config path
flashduty-runner run --config /path/to/config.yaml

# Check version
flashduty-runner version

# Manual update
flashduty-runner update
```

## Configuration Reference

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `api_key` | string | Yes | - | Flashduty API Key for authentication |
| `API_url` | string | No | `wss://api.flashcat.cloud/runner/ws` | Flashduty WebSocket endpoint |
| `name` | string | No | hostname | Runner display name |
| `labels` | []string | No | [] | Custom labels for task routing |
| `workspace_root` | string | No | `~/.flashduty-runner/workspace` | Root directory for workspace operations |
| `auto_update` | bool | No | true | Enable automatic updates |
| `permission.bash` | map | No | deny all | Glob patterns for command permission |

### Permission Patterns

The permission system uses glob patterns to control command execution:

```yaml
permission:
  bash:
    "*": "deny"              # Default: deny all commands
    "git *": "allow"         # Allow all git commands
    "kubectl get *": "allow" # Allow kubectl get
    "rm -rf *": "deny"       # Explicitly deny dangerous commands
```

**Rules:**
- Patterns are matched in order, last match wins
- `*` matches any characters
- Commands not matching any pattern are denied by default

## Built-in Labels

The runner automatically adds these labels:

| Label | Description | Example |
|-------|-------------|---------|
| `os` | Operating system | `linux`, `darwin` |
| `arch` | CPU architecture | `amd64`, `arm64` |
| `hostname` | Machine hostname | `prod-server-01` |

## Security

- **TLS**: All WebSocket connections use TLS encryption
- **API Key**: Authentication via Flashduty API Key
- **Permission**: Commands are checked against whitelist before execution
- **Path Safety**: File operations are restricted to workspace root
- **Config Protection**: Config file should have 0600 permissions

## Troubleshooting

### Connection Issues

1. Check API Key is valid
2. Verify network allows outbound WebSocket connections
3. Check firewall rules for port 443

### Permission Denied

1. Review permission patterns in config
2. Check if command matches any allow pattern
3. Verify workspace_root permissions

### Runner Not Showing Online

1. Check Flashduty console for runner status
2. Verify heartbeat is being sent (check logs)
3. Ensure API Key matches the correct account

## Development

```bash
# Clone the repository
git clone https://github.com/flashcatcloud/flashduty-runner.git
cd flashduty-runner

# Install dependencies
go mod tidy

# Build
make build

# Run tests
make test

# Run linter
make lint
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
