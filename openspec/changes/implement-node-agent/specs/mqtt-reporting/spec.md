## MODIFIED Requirements

### Requirement: Agent 行为规范

The node agent SHALL run as a systemd service and handle MQTT communication.

Agent responsibilities:
1. Publish status to `node/{mac}/status` periodically (heartbeat interval: 30 seconds)
2. Publish status immediately after installation completes
3. Subscribe to `node/{mac}/command` for remote commands
4. Handle commands including "reboot" (MVP)
5. Monitor node health (CPU, memory, disk - optional for MVP)

#### Scenario: Agent 启动并连接 MQTT

**Given** 节点已完成系统安装
**When** Agent 服务启动（systemd）
**Then** Agent 应当：
- 读取节点 MAC 地址（从网络接口获取）
- 连接到 MQTT Broker（与服务器使用相同的 Broker）
- 订阅命令主题 `/node/{mac}/command`
- 发布初始状态消息到 `node/{mac}/status`
- 启动心跳定时器（30 秒间隔）

#### Scenario: Agent 上报安装完成状态

**Given** 节点刚完成 Debian 安装
**When** Agent 首次启动（通过 preseed late_command 安装）
**Then** Agent 应当：
- 发布 MQTT 消息到 `node/{mac}/status`
- Payload: `{"status": "installed", "ip": "<当前IP>", "hostname": "<主机名>", "uptime": 0, "timestamp": "<RFC3339时间戳>"}`
- 服务器接收后将节点状态更新为 "installed"

#### Scenario: Agent 定期心跳

**Given** Agent 已启动并运行
**When** 心跳定时器触发（每 30 秒）
**Then** Agent 应当：
- 收集节点信息（IP、hostname、uptime）
- 发布到 `node/{mac}/status`
- Payload: `{"status": "installed", "ip": "...", "hostname": "...", "uptime": <运行秒数>, "timestamp": "<RFC3339时间戳>"}`

#### Scenario: Agent 接收远程命令

**Given** Agent 已订阅 `/node/{mac}/command`
**When** 接收到命令消息：
  - Payload: `{"command": "reboot", "args": {}}`
**Then** Agent 应当：
- 验证命令名称
- 执行对应的命令处理器
- 记录执行日志
- 对于 reboot 命令，执行 `systemctl reboot`

#### Scenario: Agent 重连机制

**Given** MQTT 连接断开
**When** Agent 检测到连接断开
**Then** Agent 应当：
- 自动尝试重新连接
- 重连间隔 10 秒
- 重连成功后重新订阅命令主题
- 发布初始状态消息
