## ADDED Requirements

### Requirement: Agent 主程序结构

The system SHALL provide a node agent binary that runs on installed edge nodes.

Agent SHALL:
1. Be implemented as a single Go binary
2. Target linux/arm64 architecture (RK3588)
3. Run as a systemd service
4. Connect to the same MQTT broker as the server

#### Scenario: Agent 获取节点 MAC 地址（优先使用环境变量）

**Given** Agent 启动
**When** Agent 初始化，环境变量 NF_MAC=aabbccddeeff
**Then** Agent 应当：
- 优先使用环境变量 NF_MAC 中的 MAC 地址
- 验证 MAC 格式（小写、无分隔符，12 位十六进制）
- 跳过自动检测流程
- 记录日志："Using MAC from environment: aabbccddeeff"

#### Scenario: Agent 获取节点 MAC 地址（回退到自动检测）

**Given** Agent 启动，环境变量 NF_MAC 未设置
**When** Agent 初始化
**Then** Agent 应当：
- 回退到自动检测网络接口的 MAC 地址
- 遍历所有网络接口，选择第一个非 loopback 的有效接口
- 将 MAC 地址格式化为小写、无分隔符格式（如 aabbccddeeff）
- 记录警告日志："NF_MAC not set, auto-detected MAC: aabbccddeeff"

#### Scenario: Agent 验证无效的 MAC 格式

**Given** Agent 启动，环境变量 NF_MAC=invalid-mac
**When** Agent 初始化
**Then** Agent 应当：
- 检测到 MAC 格式无效
- 记录错误日志："Invalid NF_MAC format: invalid-mac"
- 回退到自动检测
- 使用自动检测的 MAC 地址

#### Scenario: Agent 连接 MQTT Broker

**Given** Agent 已获取 MAC 地址
**When** MQTT Broker 地址配置为 192.168.1.10:1883
**Then** Agent 应当：
- 连接到指定的 MQTT Broker
- 设置 Client ID 为 `nodefoundry-agent-{mac}`
- 订阅命令主题 `/node/{mac}/command`
- 设置 QoS 0 for all messages
- 记录连接成功日志

#### Scenario: MQTT Broker 不可用时的重连

**Given** Agent 启动但 MQTT Broker 未运行
**When** Agent 尝试连接
**Then** Agent 应当：
- 进入重连循环
- 每隔 10 秒重试连接
- 记录警告日志："Failed to connect to MQTT broker, retrying..."
- 不退出程序

### Requirement: 状态上报

The agent SHALL publish node status to MQTT topic `/node/{mac}/status`.

Status message SHALL contain:
- status: "installed"
- ip: current IP address
- hostname: node hostname
- uptime: system uptime in seconds
- timestamp: RFC 3339 timestamp

#### Scenario: Agent 首次启动上报状态

**Given** Agent 刚启动并连接到 MQTT
**When** Agent 完成初始化
**Then** Agent 应当：
- 收集节点信息（IP、hostname、uptime）
- 发布 MQTT 消息到 `/node/{mac}/status`
- Payload 包含 status="installed"
- Payload 包含 timestamp（RFC 3339 格式）
- 记录日志："Initial status published"

#### Scenario: Agent 定时心跳

**Given** Agent 已启动并运行
**When** 心跳定时器触发（每 30 秒）
**Then** Agent 应当：
- 获取当前 IP 地址
- 读取系统运行时长（/proc/uptime）
- 发布心跳到 `/node/{mac}/status`
- Payload 格式：
  ```json
  {
    "status": "installed",
    "ip": "192.168.1.100",
    "hostname": "node-aabbccddeeff",
    "uptime": 3600,
    "timestamp": "2024-01-23T10:30:00Z"
  }
  ```
- 使用 debug 级别记录心跳日志

#### Scenario: 获取节点 IP 地址

**Given** Agent 需要获取节点 IP
**When** 调用 IP 获取函数
**Then** Agent 应当：
- 遍历所有网络接口
- 过滤 loopback 和 down 状态的接口
- 返回第一个有效接口的 IPv4 地址
- 如果无法获取 IP，使用 "0.0.0.0" 占位

#### Scenario: 获取系统运行时长

**Given** Agent 需要获取系统运行时长
**When** 读取 /proc/uptime 文件
**Then** Agent 应当：
- 解析文件内容（第一个值为运行秒数）
- 转换为整数秒数
- 如果读取失败，使用 0 作为默认值

### Requirement: 命令处理

The agent SHALL subscribe to `/node/{mac}/command` and execute commands.

Command message format:
- command: command name (e.g., "reboot")
- args: optional arguments object

MVP SHALL support:
- reboot: reboot the node

#### Scenario: Agent 接收 reboot 命令

**Given** Agent 已订阅 `/node/{mac}/command`
**When** 接收到 MQTT 消息：
  - Topic: `/node/aabbccddeeff/command`
  - Payload: `{"command": "reboot", "args": {}}`
**Then** Agent 应当：
- 解析 JSON payload
- 验证命令名称为 "reboot"
- 执行 `systemctl reboot` 命令
- 记录日志："Executing command: reboot"
- 命令执行后立即退出（因为系统将重启）

#### Scenario: Agent 接收未知命令

**Given** Agent 已订阅命令主题
**When** 接收到消息：
  - Payload: `{"command": "unknown", "args": {}}`
**Then** Agent 应当：
- 记录错误日志："Unknown command: unknown"
- 不执行任何操作
- 继续监听后续命令

#### Scenario: 命令消息格式错误

**Given** Agent 已订阅命令主题
**When** 接收到无效 JSON：
  - Payload: `{invalid json}`
**Then** Agent 应当：
- 记录错误日志："Failed to parse command message"
- 不崩溃
- 继续监听后续命令

#### Scenario: 命令执行失败

**Given** Agent 接收到有效命令
**When** 执行命令时发生错误（如权限不足）
**Then** Agent 应当：
- 记录错误日志："Failed to execute command: {error}"
- 不退出 Agent
- 继续监听后续命令

### Requirement: Agent 配置

The agent SHALL support configuration via environment variables.

Supported environment variables:
- NF_MQTT_BROKER: MQTT broker address (default: localhost:1883)
- NF_LOG_LEVEL: log level (default: info)
- NF_MAC: override MAC address detection (optional)
- NF_HEARTBEAT_INTERVAL: heartbeat interval in seconds (default: 30)

#### Scenario: 使用默认配置启动

**Given** 环境变量未设置
**When** Agent 启动
**Then** Agent 应当：
- 使用默认 MQTT Broker 地址：localhost:1883
- 使用默认日志级别：info
- 使用默认心跳间隔：30 秒
- 自动检测 MAC 地址

#### Scenario: 使用环境变量覆盖配置

**Given** 环境变量设置：
  - NF_MQTT_BROKER=192.168.1.10:1883
  - NF_LOG_LEVEL=debug
  - NF_HEARTBEAT_INTERVAL=60
**When** Agent 启动
**Then** Agent 应当：
- 连接到 192.168.1.10:1883
- 使用 debug 日志级别
- 每 60 秒发送一次心跳

#### Scenario: 手动指定 MAC 地址

**Given** 环境变量 NF_MAC=aabbccddeeff
**When** Agent 启动
**Then** Agent 应当：
- 验证 MAC 格式正确（12 位十六进制字符）
- 跳过自动检测
- 使用环境变量指定的 MAC 地址
- 记录日志："Using MAC address from environment: aabbccddeeff"

### Requirement: systemd 服务集成

The agent SHALL run as a systemd service with proper dependencies.

Service configuration SHALL:
- Use EnvironmentFile directive for configuration
- Start after network is online
- Restart automatically on failure
- Load environment from /etc/default/nodefoundry-agent

#### Scenario: systemd 服务启动顺序

**Given** Agent 安装为 systemd 服务
**When** 系统启动
**Then** Agent 应当：
- 等待 network-online.target 完成
- 在网络就绪后才启动
- 记录日志："Agent started"

#### Scenario: Agent 崩溃后自动恢复

**Given** Agent 正在运行
**When** Agent 进程崩溃
**Then** systemd 应当：
- 在 10 秒后自动重启 Agent
- 记录重启日志
- Agent 重启后继续正常工作

#### Scenario: 手动管理 Agent 服务

**Given** Agent 已安装
**When** 管理员执行命令：
  - `systemctl status nodefoundry-agent`
**Then** 系统应当：
- 显示服务运行状态
- 显示最近的日志输出

**When** 管理员执行命令：
  - `systemctl stop nodefoundry-agent`
**Then** Agent 应当：
- 优雅停止
- 断开 MQTT 连接
- 退出进程

### Requirement: Agent 部署

The system SHALL provide HTTP endpoints for agent deployment.

Endpoints SHALL:
- `/agent/nodefoundry-agent` - serve agent binary
- `/agent/nodefoundry-agent.service` - serve systemd service file

#### Scenario: 下载 Agent 二进制文件

**Given** NodeFoundry 服务器运行中
**When** HTTP GET 请求到 `/agent/nodefoundry-agent`
**Then** 服务器应当：
- 返回 ARM64 架构的 Agent 二进制文件
- 设置 Content-Type: application/octet-stream

#### Scenario: 下载 systemd 服务文件

**Given** NodeFoundry 服务器运行中
**When** HTTP GET 请求到 `/agent/nodefoundry-agent.service`
**Then** 服务器应当：
- 返回 systemd service 单元文件
- Content-Type: text/plain
- 文件使用 EnvironmentFile=/etc/default/nodefoundry-agent
- 不包含硬编码的环境变量

#### Scenario: preseed 自动安装 Agent

**Given** 正在进行 Debian 安装，安装通过 eth0 网卡进行 DHCP
**When** preseed late_command 执行
**Then** 安装程序应当：
- 获取当前用于 DHCP 的网卡 MAC 地址（通过 ip route 或 /sys/class/net/）
- 下载 Agent 二进制到 /usr/local/bin/nodefoundry-agent
- 设置可执行权限（chmod +x）
- 下载 systemd 服务文件到 /etc/systemd/system/nodefoundry-agent.service
- 创建环境变量文件 /etc/default/nodefoundry-agent，包含：
  - NF_MAC=<DHCP网卡的MAC地址，格式化为小写无分隔符>
  - NF_MQTT_BROKER=${NF_SERVER_ADDR}:1883
  - NF_LOG_LEVEL=info
  - NF_HEARTBEAT_INTERVAL=30
- 启用服务（systemctl enable）
- 记录安装日志到 /var/log/nodefoundry-agent-install.log

### Requirement: ARM64 编译支持

The agent SHALL be compiled for linux/arm64 architecture (RK3588).

#### Scenario: 编译 ARM64 版本

**Given** 源代码已准备
**When** 执行编译命令：
  - `GOOS=linux GOARCH=arm64 go build -o bin/nodefoundry-agent`
**Then** 系统应当：
- 生成 ARM64 二进制文件
- 文件可在 RK3588 Debian 系统上运行
