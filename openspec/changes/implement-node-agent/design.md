# Design: 边缘节点 Agent 实现

## Context

Agent 运行在已安装 Debian 系统的边缘节点上，作为 systemd 服务持续运行。它通过 MQTT 与 NodeFoundry 服务器通信，负责节点状态上报和命令执行。

**约束条件**:
- 节点硬件：RK3588 (ARM64, 8GB RAM)
- 网络环境：局域网，MQTT Broker 可达
- 部署方式：通过 preseed `late_command` 自动安装
- 系统依赖：Debian 12 (Bookworm)

**利益相关者**:
- 服务器端：需要接收节点状态、发送命令
- 节点：需要上报状态、执行命令
- 管理员：需要监控节点状态、远程控制节点

## Goals / Non-Goals

**Goals**:
- 实现轻量级 Agent，内存占用 < 50MB
- 支持状态上报和心跳（30 秒间隔）
- 支持远程命令执行（MVP: reboot）
- 支持 systemd 服务集成
- 目标架构：linux/arm64 (RK3588)

**Non-Goals**:
- 不支持命令执行结果上报（MVP 阶段）
- 不支持本地配置文件（所有配置通过环境变量/命令行）
- 不支持 TLS/认证（MVP 阶段）
- 不支持模型任务执行（未来扩展）

## Decisions

### Decision 1: Agent 架构设计

**选择**: 单一二进制文件 + MQTT 客户端模式

**理由**:
- Go 编译为单一二进制，部署简单
- 使用 `github.com/eclipse/paho.mqtt.golang` MQTT 客户端库
- 与服务器端使用相同的 MQTT 库，保持一致性

**替代方案**:
- 使用系统服务 + 独立 MQTT 客户端（如 mosquitto-clients + shell 脚本）
  - 优点：更简单
  - 缺点：功能扩展性差，错误处理复杂
- 使用 gRPC/HTTP 轮询
  - 优点：无需 MQTT Broker
  - 缺点：服务器需要维持连接状态，扩展性差

### Decision 2: 状态上报格式

**选择**: JSON payload over MQTT

**Topic**: `/node/{mac}/status`
**Payload**:
```json
{
  "status": "installed",
  "ip": "192.168.1.100",
  "hostname": "node-aabbccddeeff",
  "uptime": 3600,
  "timestamp": "2024-01-23T10:30:00Z"
}
```

**理由**:
- JSON 易于解析和扩展
- 与服务器端现有的消息格式一致
- 支持结构化日志

### Decision 3: 命令处理设计

**选择**: 订阅命令主题 + 命令处理器模式

**Topic**: `/node/{mac}/command`
**Payload**:
```json
{
  "command": "reboot",
  "args": {}
}
```

**命令处理器接口**:
```go
type CommandHandler interface {
    Name() string
    Execute(ctx context.Context, args map[string]interface{}) error
}
```

**理由**:
- 易于扩展新命令
- 每个命令独立处理，降低耦合
- 支持命令参数传递

### Decision 4: MAC 地址获取

**选择**: 优先使用环境变量，回退到自动检测

**优先级**:
1. **环境变量 `NF_MAC`**（推荐）：由 preseed 注入，保证与 DHCP 使用的一致性
2. **自动检测**（回退）：遍历网络接口，选择第一个有效接口

**方法**:
```go
func getMAC(envMAC string) string {
    if envMAC != "" {
        return envMAC
    }
    // 回退到自动检测：遍历网络接口，选择第一个非 loopback 的有效接口
}
```

**preseed 集成**:
```bash
# late_command 中获取当前用于 DHCP 的网卡 MAC
# 通过 /sys/class/net/ 或 ip命令获取
DHCP_MAC=$(ip route | grep default | awk '{print $5}' | xargs cat /sys/class/net/$0/address)

# 写入环境变量文件
in-target sh -c 'echo "NF_MAC='"$DHCP_MAC"'" >> /etc/default/nodefoundry-agent'
```

**理由**:
- 环境变量确保 MAC 与 DHCP 一致性（多网卡环境的关键）
- 自动检测作为回退，兼容手动部署场景
- 与服务器端 DHCP 发现一致（基于 MAC 识别节点）

**风险**: 环境变量格式错误或无效
**缓解**: Agent 验证 MAC 格式，记录警告日志

### Decision 5: systemd 服务配置

**选择**: 使用 `EnvironmentFile` 的 systemd service 单元

**systemd service 文件**（静态，通过 API 下载）:
```ini
[Unit]
Description=NodeFoundry Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nodefoundry-agent
Restart=always
RestartSec=10
EnvironmentFile=/etc/default/nodefoundry-agent

[Install]
WantedBy=multi-user.target
```

**环境变量文件**（动态生成）:
```bash
# /etc/default/nodefoundry-agent
NF_MQTT_BROKER=192.168.1.10:1883
NF_LOG_LEVEL=info
NF_HEARTBEAT_INTERVAL=30
```

**preseed 集成**:
```bash
# late_command 中动态生成环境变量文件
in-target sh -c 'echo "NF_MQTT_BROKER=${NF_SERVER_ADDR}:1883" > /etc/default/nodefoundry-agent'
in-target sh -c 'echo "NF_LOG_LEVEL=info" >> /etc/default/nodefoundry-agent'
```

**理由**:
- `network-online.target` 确保网络就绪后再启动
- `Restart=always` 保证服务崩溃后自动恢复
- `EnvironmentFile` 支持动态配置，无需修改 systemd 单元文件
- preseed 阶段通过 `${NF_SERVER_ADDR}` 变量注入正确的 MQTT 地址

### Decision 6: ARM64 编译

**选择**: Go 交叉编译 + Makefile 目标

**目标架构**: linux/arm64 (RK3588)

**编译命令**:
```bash
GOOS=linux GOARCH=arm64 go build -o bin/nodefoundry-agent
```

**理由**:
- Go 原生支持交叉编译
- MVP 阶段仅需要支持 RK3588 (ARM64) 平台
- 简化部署流程，单一二进制文件

## Component Design

### Agent 主程序 (`cmd/nodefoundry-agent/main.go`)

```go
type Agent struct {
    mac        string
    broker     string
    client     mqtt.Client
    logger     *zap.Logger
    heartbeat  *time.Ticker
    handlers   map[string]CommandHandler
}

func (a *Agent) Start(ctx context.Context) error {
    // 1. 获取 MAC 地址
    // 2. 连接 MQTT Broker
    // 3. 订阅命令主题
    // 4. 发布初始状态
    // 5. 启动心跳
}

func (a *Agent) publishStatus() {
    // 收集节点信息（IP、hostname、uptime）
    // 发布到 /node/{mac}/status
}

func (a *Agent) onCommand(msg mqtt.Message) {
    // 解析命令
    // 查找处理器
    // 执行命令
}
```

### 命令处理器 (`internal/agent/commands/`)

```go
type RebootCommand struct{}

func (c *RebootCommand) Name() string {
    return "reboot"
}

func (c *RebootCommand) Execute(ctx context.Context, args map[string]interface{}) error {
    // 执行 systemctl reboot
    return exec.Command("systemctl", "reboot").Start()
}
```

### HTTP 端点 (`internal/api/handler.go`)

```go
func (h *Handler) GetAgentBinary(c *gin.Context) {
    // 返回 ARM64 架构的 Agent 二进制文件
}

func (h *Handler) GetAgentServiceFile(c *gin.Context) {
    // 返回 systemd service 文件模板
    // 支持变量替换：${NF_MQTT_BROKER}
}
```

### preseed 集成 (`internal/ipxe/preseed.go`)

```go
// 在 late_command 中添加 Agent 安装
lateCommand := fmt.Sprintf(`
    # 获取当前用于 DHCP 的网卡 MAC 地址
    DHCP iface=$(ip route | grep default | awk '{print $5}')
    DHCP_MAC=$(cat /sys/class/net/${DHCP_iface}/address | tr -d ':')
    echo "Detected DHCP MAC: ${DHCP_MAC}" > /target/var/log/nodefoundry-agent-install.log

    # 下载并安装 Agent
    in-target wget http://%s/agent/nodefoundry-agent -O /usr/local/bin/nodefoundry-agent &&
    in-target chmod +x /usr/local/bin/nodefoundry-agent &&
    in-target wget http://%s/agent/nodefoundry-agent.service -O /etc/systemd/system/nodefoundry-agent.service &&

    # 生成环境变量文件，包含 DHCP MAC
    in-target sh -c 'echo "NF_MAC=%s" > /etc/default/nodefoundry-agent' &&
    in-target sh -c 'echo "NF_MQTT_BROKER=%s:1883" >> /etc/default/nodefoundry-agent' &&
    in-target sh -c 'echo "NF_LOG_LEVEL=info" >> /etc/default/nodefoundry-agent' &&
    in-target sh -c 'echo "NF_HEARTBEAT_INTERVAL=30" >> /etc/default/nodefoundry-agent' &&

    # 启用服务
    in-target systemctl enable nodefoundry-agent.service
`, serverAddr, serverAddr, "${DHCP_MAC}", serverAddr)
```

**说明**：
- `ip route | grep default` 获取默认路由的网卡名
- 从 `/sys/class/net/<iface>/address` 读取 MAC 地址
- `tr -d ':'` 移除 MAC 地址中的冒号，转换为小写无分隔符格式
- `${NF_SERVER_ADDR}` 在 preseed 生成时被替换为实际的服务器地址
- Agent 使用服务器地址作为 MQTT Broker（假设 MQTT Broker 运行在同一服务器上）
- 如果需要独立的 MQTT Broker 地址，可以在 preseed 生成时传入额外参数

## Data Flow

### Agent 启动流程
```
1. systemd 启动 Agent
2. Agent 获取本机 MAC 地址
3. 连接 MQTT Broker (192.168.1.10:1883)
4. 订阅 /node/{mac}/command
5. 发布初始状态到 /node/{mac}/status
6. 启动心跳定时器 (30 秒)
```

### 状态上报流程
```
1. 心跳定时器触发
2. Agent 收集节点信息：
   - IP 地址（通过 net.InterfaceAddrs）
   - 主机名（通过 os.Hostname）
   - 运行时长（通过读取 /proc/uptime）
3. 构建 JSON payload
4. Publish 到 /node/{mac}/status (QoS 0)
```

### 命令执行流程
```
1. 管理员通过 API 发送命令（可选，未来功能）
2. 服务器 publish 到 /node/{mac}/command
3. Agent 接收消息
4. 解析命令和参数
5. 查找对应的 CommandHandler
6. Execute() 执行命令
7. 记录日志
```

## Risks / Trade-offs

### Risk 1: MQTT Broker 不可用
**影响**: Agent 无法启动或无法上报状态
**缓解**:
- MQTT 连接断开时进入重连循环
- 心跳失败不导致 Agent 崩溃
- 日志记录连接状态

### Risk 2: MAC 地址获取错误
**影响**: 状态上报到错误的主题，服务器无法识别节点
**缓解**:
- 启动时记录获取到的 MAC 地址
- 提供环境变量 `NF_MAC` 覆盖自动检测
- 验证 MAC 地址格式

### Risk 3: 命令执行失败
**影响**: 命令无法执行或节点状态异常
**缓解**:
- 每个命令处理器独立错误处理
- 记录命令执行日志
- 关键命令（如 reboot）执行前验证

### Trade-off 1: 心跳间隔选择
**30 秒**:
- 优点: 及时发现节点离线
- 缺点: 网络开销略高
- **决策**: 30 秒是平衡点

### Trade-off 2: QoS 级别
**QoS 0**:
- 优点: 性能高，无需确认
- 缺点: 可能丢失消息
- **决策**: 心跳消息可丢失，最终一致性可接受

## Migration Plan

**部署步骤**:

1. **编译 Agent 二进制**
   ```bash
   make build-agent
   ```

2. **部署 Agent 文件**
   - 将二进制文件放到 `public/agent/` 目录
   - 或通过 API handler 动态返回

3. **更新 preseed 模板**
   - 在 `late_command` 中添加 Agent 安装命令

4. **测试安装流程**
   - 启动新节点
   - 触发安装
   - 验证 Agent 自动安装并启动

**回滚计划**:
- 从 preseed 中移除 Agent 安装命令
- 已安装节点：`systemctl disable nodefoundry-agent`

## Open Questions

1. **Agent 配置**: 是否需要本地配置文件支持？
   - 当前决策：否，使用环境变量
   - 未来可能：支持 `/etc/nodefoundry-agent/config.yaml`

2. **命令响应**: 是否需要上报命令执行结果？
   - 当前决策：否，仅日志记录
   - 未来可能：publish 到 `/node/{mac}/command/result`

3. **版本管理**: 如何处理 Agent 升级？
   - 当前决策：手动重新安装
   - 未来可能：支持 `/node/{mac}/command: update`
