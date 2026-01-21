# Flashduty Runner

[English](README.md) | 中文

Flashduty Runner 是一个轻量级的代理程序，运行在您的环境中，代表 Flashduty AI SRE 平台执行命令和访问资源。

## 功能特性

- **安全连接**：通过 WebSocket 使用 API Key 认证连接 Flashduty 云端
- **工作区操作**：执行 bash 命令、读写文件、使用 grep/glob 搜索
- **权限控制**：基于 glob 模式的命令白名单/黑名单，确保安全
- **标签路由**：为 runner 打标签以实现任务路由（如 `k8s`、`production`）
- **自动更新**：支持版本检查和自动二进制更新
- **MCP 代理**：通过 runner 连接内部 MCP 服务器

## 快速开始

### 安装

从 Release 页面下载适合您平台的最新版本：

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

### Docker 方式

```bash
docker run -d \
  --name flashduty-runner \
  -e FLASHDUTY_API_KEY=your_api_key \
  -e FLASHDUTY_RUNNER_NAME=my-runner \
  -v /var/flashduty/workspace:/workspace \
  registry.flashcat.cloud/public/flashduty-runner
```

### 配置

在 `~/.flashduty-runner/config.yaml` 创建配置文件：

```yaml
# Flashduty 控制台获取的 API Key（必填）
api_key: "fk_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# Flashduty WebSocket 端点
api_url: "wss://api.flashcat.cloud/runner/ws"

# Runner 标识名称
name: "prod-k8s-runner"

# 任务路由标签
labels:
  - k8s
  - production
  - mysql

# 工作区根目录
workspace_root: "/var/flashduty/workspace"

# 自动更新设置
auto_update: true

# 命令权限控制（glob 模式匹配）
permission:
  bash:
    "*": "deny"                  # 默认拒绝所有
    "git *": "allow"
    "kubectl get *": "allow"
    "kubectl describe *": "allow"
    "kubectl logs *": "allow"
    "grep *": "allow"
    "cat *": "allow"
    "ls *": "allow"
```

### 运行

```bash
# 启动 runner
flashduty-runner run

# 使用自定义配置路径启动
flashduty-runner run --config /path/to/config.yaml

# 查看版本
flashduty-runner version

# 手动更新
flashduty-runner update
```

## 配置参考

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `api_key` | string | 是 | - | Flashduty API Key 用于认证 |
| `api_url` | string | 否 | `wss://api.flashcat.cloud/runner/ws` | Flashduty WebSocket 端点 |
| `name` | string | 否 | 主机名 | Runner 显示名称 |
| `labels` | []string | 否 | [] | 任务路由的自定义标签 |
| `workspace_root` | string | 否 | `~/.flashduty-runner/workspace` | 工作区操作的根目录 |
| `auto_update` | bool | 否 | true | 启用自动更新 |
| `permission.bash` | map | 否 | 全部拒绝 | 命令权限的 glob 模式 |

### 权限模式

权限系统使用 glob 模式来控制命令执行：

```yaml
permission:
  bash:
    "*": "deny"              # 默认：拒绝所有命令
    "git *": "allow"         # 允许所有 git 命令
    "kubectl get *": "allow" # 允许 kubectl get
    "rm -rf *": "deny"       # 明确拒绝危险命令
```

**规则：**
- 模式按顺序匹配，最后匹配的规则生效
- `*` 匹配任意字符
- 未匹配任何模式的命令默认被拒绝

## 内置标签

Runner 会自动添加以下标签：

| 标签 | 说明 | 示例 |
|------|------|------|
| `os` | 操作系统 | `linux`、`darwin` |
| `arch` | CPU 架构 | `amd64`、`arm64` |
| `hostname` | 主机名 | `prod-server-01` |

## 安全性

- **TLS**：所有 WebSocket 连接使用 TLS 加密
- **API Key**：通过 Flashduty API Key 进行认证
- **权限控制**：命令执行前会检查白名单
- **路径安全**：文件操作限制在工作区根目录内
- **配置保护**：配置文件应设置 0600 权限

## 故障排除

### 连接问题

1. 检查 API Key 是否有效
2. 验证网络是否允许出站 WebSocket 连接
3. 检查防火墙规则是否允许 443 端口

### 权限被拒绝

1. 检查配置中的权限模式
2. 确认命令是否匹配某个允许模式
3. 验证 workspace_root 的权限

### Runner 未显示在线

1. 在 Flashduty 控制台检查 runner 状态
2. 验证心跳是否正在发送（检查日志）
3. 确保 API Key 对应正确的账户

## 开发

```bash
# 克隆仓库
git clone https://github.com/flashcatcloud/flashduty-runner.git
cd flashduty-runner

# 安装依赖
go mod tidy

# 构建
make build

# 运行测试
make test

# 运行 linter
make lint
```

## 开源协议

Apache License 2.0 - 详见 [LICENSE](LICENSE)。
