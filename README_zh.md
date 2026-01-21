# Flashduty Runner

<p align="center">
  <a href="https://github.com/flashcatcloud/flashduty-runner/actions/workflows/go.yml"><img src="https://github.com/flashcatcloud/flashduty-runner/actions/workflows/go.yml/badge.svg" alt="Go"></a>
  <a href="https://github.com/flashcatcloud/flashduty-runner/actions/workflows/lint.yml"><img src="https://github.com/flashcatcloud/flashduty-runner/actions/workflows/lint.yml/badge.svg" alt="Lint"></a>
  <a href="https://github.com/flashcatcloud/flashduty-runner/releases"><img src="https://img.shields.io/github/v/release/flashcatcloud/flashduty-runner" alt="Release"></a>
  <a href="https://goreportcard.com/report/github.com/flashcatcloud/flashduty-runner"><img src="https://goreportcard.com/badge/github.com/flashcatcloud/flashduty-runner" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/flashcatcloud/flashduty-runner" alt="License"></a>
</p>

<p align="center">
  <a href="README.md">English</a> | 中文
</p>

Flashduty Runner 是一个轻量级、安全的代理程序，运行在您的环境中，代表 [Flashduty](https://flashcat.cloud) AI SRE 平台执行命令和访问资源。

## 工作原理

```
┌──────────────────┐       WebSocket (TLS)       ┌────────────────────┐
│  Flashduty AI    │ ◄─────────────────────────► │  Flashduty Runner  │
│  SRE 平台        │                             │  (您的服务器)      │
└──────────────────┘                             └────────────────────┘
                                                          │
                                                          ▼
                                                 ┌────────────────────┐
                                                 │ • 执行命令         │
                                                 │ • 读写文件         │
                                                 │ • MCP 工具调用     │
                                                 └────────────────────┘
```

Runner 与 Flashduty 云端建立持久的 WebSocket 连接，接收任务请求，在本地执行并返回结果。

## 安全性

**所有代码完全开源** - 您可以审计每一行代码，确切了解 Runner 的行为。

### 多层安全设计

| 层级 | 保护措施 |
|------|----------|
| **传输层** | TLS 加密的 WebSocket，API Key 认证 |
| **命令执行** | Shell 解析防止注入攻击（如 `cmd1; cmd2`） |
| **权限控制** | 可配置的 glob 模式命令白名单/黑名单 |
| **文件系统** | 操作限制在工作区目录内，防止符号链接逃逸 |

### 权限配置

Runner 使用 **glob 模式匹配** 进行命令权限控制。您可以完全控制哪些命令可以被执行。

#### 方案一：严格模式（推荐用于共享环境）

仅明确允许特定命令：

```yaml
permission:
  bash:
    "*": "deny"                  # 默认拒绝所有
    "kubectl get *": "allow"
    "kubectl describe *": "allow"
    "kubectl logs *": "allow"
    "cat *": "allow"
    "ls *": "allow"
```

#### 方案二：信任模式（用于专属/隔离环境）

如果 Runner 部署在专门用于 AI 运维的隔离环境中，您可以选择信任 AI 模型的判断：

```yaml
permission:
  bash:
    "*": "allow"                 # 信任 AI 模型
    "rm -rf /": "deny"           # 如需要可阻止灾难性命令
```

此模式适用于：
- Runner 运行在隔离的 VM/容器中，影响范围有限
- 您信任 AI 模型的能力，希望获得最大灵活性
- 快速响应事件比限制权限更重要

#### 方案三：只读模式（仅用于监控）

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

## 快速开始

### 二进制安装

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

### Docker 安装

```bash
docker run -d \
  --name flashduty-runner \
  -e FLASHDUTY_RUNNER_API_KEY=your_api_key \
  -e FLASHDUTY_RUNNER_NAME=my-runner \
  -v /var/flashduty/workspace:/workspace \
  ghcr.io/flashcatcloud/flashduty-runner:latest
```

### 配置

创建 `~/.flashduty-runner/config.yaml`：

```yaml
# Flashduty 控制台获取的 API Key（必填）
api_key: "fk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# Runner 显示名称（可选，默认为主机名）
name: "prod-k8s-runner"

# 任务路由标签（可选）
labels:
  - k8s
  - production

# 工作区根目录（可选）
workspace_root: "/var/flashduty/workspace"

# 命令权限（参见安全性章节的选项）
permission:
  bash:
    "*": "deny"
    "kubectl get *": "allow"
    "kubectl describe *": "allow"
    "kubectl logs *": "allow"
```

### 运行

```bash
# 启动 runner
flashduty-runner run

# 使用自定义配置启动
flashduty-runner run --config /path/to/config.yaml

# 查看版本
flashduty-runner version
```

### Systemd 服务（Linux）

创建 `/etc/systemd/system/flashduty-runner.service`：

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

## 配置参考

| 字段 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `api_key` | 是 | - | Flashduty API Key |
| `api_url` | 否 | `wss://api.flashcat.cloud/runner/ws` | WebSocket 端点 |
| `name` | 否 | 主机名 | Runner 显示名称 |
| `labels` | 否 | [] | 任务路由标签 |
| `workspace_root` | 否 | `~/.flashduty-runner/workspace` | 工作区目录 |
| `permission.bash` | 否 | 全部拒绝 | 命令权限规则 |
| `log.level` | 否 | `info` | 日志级别：debug, info, warn, error |

### 环境变量

所有选项都可以通过 `FLASHDUTY_RUNNER_` 前缀的环境变量设置：

```bash
FLASHDUTY_RUNNER_API_KEY=fk_xxx
FLASHDUTY_RUNNER_NAME=my-runner
FLASHDUTY_RUNNER_WORKSPACE_ROOT=/workspace
```

### 内置标签

Runner 会自动添加以下标签用于路由：

- `os:linux` / `os:darwin` / `os:windows`
- `arch:amd64` / `arch:arm64`
- `hostname:<主机名>`

## 故障排除

### 连接问题

| 症状 | 原因 | 解决方案 |
|------|------|----------|
| `failed to connect` | 网络问题 | 检查防火墙是否允许出站端口 443 |
| `authentication failed` | API Key 无效 | 在 Flashduty 控制台验证 API Key |
| Runner 未显示在线 | 连接断开 | 检查日志，验证 API Key 是否匹配账户 |

```bash
# 测试连通性
curl -v https://api.flashcat.cloud/health

# 检查 runner 日志
journalctl -u flashduty-runner -f
```

### 权限问题

| 症状 | 原因 | 解决方案 |
|------|------|----------|
| `command denied` | 命令不在白名单中 | 在 `permission.bash` 中添加模式 |
| `path escapes workspace` | 路径遍历被阻止 | 使用 `workspace_root` 内的路径 |

**权限模式规则：**
- 模式按顺序匹配，**最后匹配的规则生效**
- `*` 匹配任意字符
- 空配置默认拒绝所有

### 调试模式

启用调试日志以查看详细的权限决策：

```yaml
log:
  level: "debug"
```

## 贡献

欢迎贡献！请查看 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 开源协议

Apache License 2.0 - 详见 [LICENSE](LICENSE)。

---

<p align="center">
  由 <a href="https://flashcat.cloud">Flashcat</a> 用 ❤️ 打造
</p>
