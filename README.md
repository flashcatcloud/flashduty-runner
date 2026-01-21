# Flashduty Runner

<p align="center">
  <a href="https://github.com/flashcatcloud/flashduty-runner/actions/workflows/go.yml"><img src="https://github.com/flashcatcloud/flashduty-runner/actions/workflows/go.yml/badge.svg" alt="Go"></a>
  <a href="https://github.com/flashcatcloud/flashduty-runner/actions/workflows/lint.yml"><img src="https://github.com/flashcatcloud/flashduty-runner/actions/workflows/lint.yml/badge.svg" alt="Lint"></a>
  <a href="https://github.com/flashcatcloud/flashduty-runner/releases"><img src="https://img.shields.io/github/v/release/flashcatcloud/flashduty-runner" alt="Release"></a>
  <a href="https://goreportcard.com/report/github.com/flashcatcloud/flashduty-runner"><img src="https://goreportcard.com/badge/github.com/flashcatcloud/flashduty-runner" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/flashcatcloud/flashduty-runner" alt="License"></a>
</p>

<p align="center">
  English | <a href="README_zh.md">中文</a>
</p>

Flashduty Runner is a lightweight, secure agent that runs in your environment to execute commands and access resources on behalf of [Flashduty](https://flashcat.cloud) AI SRE platform.

## How It Works

```
┌──────────────────┐       WebSocket (TLS)       ┌────────────────────┐
│  Flashduty AI    │ ◄─────────────────────────► │  Flashduty Runner  │
│  SRE Platform    │                             │  (Your Server)     │
└──────────────────┘                             └────────────────────┘
                                                          │
                                                          ▼
                                                 ┌────────────────────┐
                                                 │ • Execute Commands │
                                                 │ • Read/Write Files │
                                                 │ • MCP Tool Calls   │
                                                 └────────────────────┘
```

The runner establishes a persistent WebSocket connection to Flashduty cloud, receives task requests, executes them locally, and returns results.

## Security

**All code is open source** - you can audit every line of code to verify exactly what the runner does.

### Multi-layer Security Design

| Layer | Protection |
|-------|------------|
| **Transport** | TLS-encrypted WebSocket, API Key authentication |
| **Command Execution** | Shell parsing to prevent injection attacks (e.g., `cmd1; cmd2`) |
| **Permission Control** | Configurable glob-based command whitelist/blacklist |
| **File System** | Operations sandboxed to workspace root, symlink escape protection |

### Permission Configuration

The runner uses **glob pattern matching** for command permissions. You have full control over what commands can be executed.

#### Option 1: Strict Mode (Recommended for shared environments)

Only allow specific commands explicitly:

```yaml
permission:
  bash:
    "*": "deny"                  # Deny all by default
    "kubectl get *": "allow"
    "kubectl describe *": "allow"
    "kubectl logs *": "allow"
    "cat *": "allow"
    "ls *": "allow"
```

#### Option 2: Trust Mode (For dedicated/isolated environments)

If the runner is deployed in an isolated environment dedicated to AI operations, you can choose to trust the AI model's judgment:

```yaml
permission:
  bash:
    "*": "allow"                 # Trust AI model
    "rm -rf /": "deny"           # Block catastrophic commands if desired
```

This mode is suitable when:
- The runner runs in an isolated VM/container with limited blast radius
- You trust the AI model's capabilities and want maximum flexibility
- Quick incident response is more important than restrictive permissions

#### Option 3: Read-Only Mode (For monitoring only)

```yaml
permission:
  bash:
    "*": "deny"
    "cat *": "allow"
    "head *": "allow"
    "tail *": "allow"
    "ls *": "allow"
    "grep *": "allow"
    "ps *": "allow"
    "df *": "allow"
    "free *": "allow"
```

## Quick Start

### Binary Installation

```bash
# Linux (amd64)
curl -LO https://github.com/flashcatcloud/flashduty-runner/releases/latest/download/flashduty-runner_Linux_x86_64.tar.gz
tar -xzf flashduty-runner_Linux_x86_64.tar.gz
sudo mv flashduty-runner /usr/local/bin/

# Linux (arm64)
curl -LO https://github.com/flashcatcloud/flashduty-runner/releases/latest/download/flashduty-runner_Linux_arm64.tar.gz
tar -xzf flashduty-runner_Linux_arm64.tar.gz
sudo mv flashduty-runner /usr/local/bin/

# macOS (Apple Silicon)
curl -LO https://github.com/flashcatcloud/flashduty-runner/releases/latest/download/flashduty-runner_Darwin_arm64.tar.gz
tar -xzf flashduty-runner_Darwin_arm64.tar.gz
sudo mv flashduty-runner /usr/local/bin/

# macOS (Intel)
curl -LO https://github.com/flashcatcloud/flashduty-runner/releases/latest/download/flashduty-runner_Darwin_x86_64.tar.gz
tar -xzf flashduty-runner_Darwin_x86_64.tar.gz
sudo mv flashduty-runner /usr/local/bin/
```

### Docker Installation

```bash
docker run -d \
  --name flashduty-runner \
  -e FLASHDUTY_RUNNER_API_KEY=your_api_key \
  -e FLASHDUTY_RUNNER_NAME=my-runner \
  -v /var/flashduty/workspace:/workspace \
  ghcr.io/flashcatcloud/flashduty-runner:latest
```

### Configuration

Create `~/.flashduty-runner/config.yaml`:

```yaml
# API Key from Flashduty Console (required)
api_key: "fk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# Runner display name (optional, defaults to hostname)
name: "prod-k8s-runner"

# Labels for task routing (optional)
labels:
  - k8s
  - production

# Workspace root directory (optional)
workspace_root: "/var/flashduty/workspace"

# Command permissions (see Security section for options)
permission:
  bash:
    "*": "deny"
    "kubectl get *": "allow"
    "kubectl describe *": "allow"
    "kubectl logs *": "allow"
```

### Running

```bash
# Start the runner
flashduty-runner run

# Start with custom config
flashduty-runner run --config /path/to/config.yaml

# Check version
flashduty-runner version
```

### Systemd Service (Linux)

Create `/etc/systemd/system/flashduty-runner.service`:

```ini
[Unit]
Description=Flashduty Runner
After=network.target

[Service]
Type=simple
User=flashduty
ExecStart=/usr/local/bin/flashduty-runner run
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now flashduty-runner
```

## Configuration Reference

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `api_key` | Yes | - | Flashduty API Key |
| `api_url` | No | `wss://api.flashcat.cloud/runner/ws` | WebSocket endpoint |
| `name` | No | hostname | Runner display name |
| `labels` | No | [] | Labels for task routing |
| `workspace_root` | No | `~/.flashduty-runner/workspace` | Workspace directory |
| `permission.bash` | No | deny all | Command permission rules |
| `log.level` | No | `info` | Log level: debug, info, warn, error |

### Environment Variables

All options can be set via environment variables with `FLASHDUTY_RUNNER_` prefix:

```bash
FLASHDUTY_RUNNER_API_KEY=fk_xxx
FLASHDUTY_RUNNER_NAME=my-runner
FLASHDUTY_RUNNER_WORKSPACE_ROOT=/workspace
```

### Built-in Labels

The runner automatically adds these labels for routing:

- `os:linux` / `os:darwin` / `os:windows`
- `arch:amd64` / `arch:arm64`
- `hostname:<machine-hostname>`

## Troubleshooting

### Connection Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| `failed to connect` | Network issue | Check firewall allows outbound port 443 |
| `authentication failed` | Invalid API Key | Verify API Key in Flashduty console |
| Runner not showing online | Connection dropped | Check logs, verify API Key matches account |

```bash
# Test connectivity
curl -v https://api.flashcat.cloud/health

# Check runner logs
journalctl -u flashduty-runner -f
```

### Permission Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| `command denied` | Command not in whitelist | Add pattern to `permission.bash` |
| `path escapes workspace` | Path traversal blocked | Use paths within `workspace_root` |

**Permission Pattern Rules:**
- Patterns are matched in order, **last match wins**
- `*` matches any characters
- Empty config defaults to deny all

### Debug Mode

Enable debug logging to see detailed permission decisions:

```yaml
log:
  level: "debug"
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache License 2.0 - see [LICENSE](LICENSE).

---

<p align="center">
  Made with ❤️ by <a href="https://flashcat.cloud">Flashcat</a>
</p>
