# Capability: MQTT Status Reporting

## ADDED Requirements

### Requirement: 连接 Mosquitto Broker

The system SHALL connect to Mosquitto MQTT Broker as a client.

MQTT connection SHALL:
1. Use QoS 0 for status transmission
2. Implement automatic reconnection mechanism
3. Subscribe to node status report topics

#### Scenario: 系统启动时连接到 MQTT Broker

**Given** Mosquitto Broker 运行在 localhost:1883
**When** NodeFoundry 启动
**Then** 系统应当：
- 成功连接到 Broker
- 订阅 `node/+/status` 主题（+ 为通配符）
- 记录连接成功日志

#### Scenario: MQTT Broker 不可用

**Given** Mosquitto Broker 未运行
**When** NodeFoundry 尝试连接
**Then** 系统应当：
- 记录错误日志
- 进入重连循环
- 不影响其他服务（HTTP、DHCP）运行

### Requirement: 处理节点上报的状态消息

The system SHALL process status messages reported by node agents via MQTT.

Node status message format:
- Topic: `node/{mac}/status`
- Payload: JSON containing status, ip and other fields

#### Scenario: 接收节点安装完成状态

**Given** 节点（MAC: AABBCCDDEEFF）状态为 "installing"
**When** Agent 上报 MQTT 消息：
  - Topic: `node/aabbccddeeff/status`
  - Payload: `{"status": "installed", "ip": "192.168.1.100", "hostname": "node-aabbccddeeff"}`
**Then** 系统应当：
- 解析 JSON payload
- 验证状态转换合法性（installing → installed）
- 更新数据库中的节点状态为 "installed"
- 更新节点 IP 地址和主机名
- 更新 LastHeartbeat 时间戳
- 记录日志："Node aabbccddeeff reported status: installed"

#### Scenario: 接收到无效的状态转换

**Given** 节点（MAC: AABBCCDDEEFF）状态为 "discovered"
**When** MQTT 接收到消息：
  - Topic: `node/aabbccddeeff/status`
  - Payload: `{"status": "installed"}`
**Then** 系统应当：
- 拒绝状态转换（非法跳过 installing）
- 记录警告日志："Invalid status transition for node aabbccddeeff: discovered -> installed"
- 不更新数据库

#### Scenario: 接收到未知节点的状态消息

**Given** 节点（MAC: FFFFFFFF0000）不存在于数据库中
**When** MQTT 接收到消息：
  - Topic: `node/ffffffff0000/status`
  - Payload: `{"status": "installed"}`
**Then** 系统应当：
- 记录警告日志："Received status from unknown node ffffffff0000"
- 不创建新节点（MQTT 状态仅用于已注册节点）

#### Scenario: 接收到格式错误的 JSON

**Given** MQTT 接收到消息：
  - Topic: `node/aabbccddeeff/status`
  - Payload: `{invalid json}`
**Then** 系统应当：
- 记录错误日志："Failed to parse status message: ..."
- 不崩溃，继续处理后续消息

### Requirement: 实现 MQTT 心跳机制监控节点在线状态

The system SHALL implement MQTT heartbeat mechanism to monitor node online status.

#### Scenario: Agent 定期发送心跳

**Given** 节点已安装并运行 agent
**When** Agent 定期上报 MQTT 消息：
  - Topic: `node/aabbccddeeff/status`
  - Payload: `{"status": "installed", "ip": "192.168.1.100", "uptime": 3600}`
**Then** 系统应当：
- 更新节点的 LastHeartbeat 为当前时间
- 更新节点运行时长（uptime）
- 记录心跳日志（debug 级别）

#### Scenario: 检测离线节点（可选）

**Given** 节点超过 5 分钟未上报状态
**When** 系统执行健康检查
**Then** 系统应当（可选）：
- 标记节点为 "stale" 或 "offline"（如果引入此状态）
- 发送告警通知

### Requirement: Agent 行为规范

The node agent SHALL run as a systemd service and handle MQTT communication.

Agent responsibilities:
1. Publish status to `node/{mac}/status` periodically (heartbeat)
2. Publish status immediately after installation completes
3. Monitor node health (CPU, memory, disk - optional for MVP)

#### Scenario: Agent 启动并连接 MQTT

**Given** 节点已完成系统安装
**When** Agent 服务启动（systemd）
**Then** Agent 应当：
- 读取节点 MAC 地址（从网络接口获取）
- 连接到 MQTT Broker
- 发布初始状态消息到 `node/{mac}/status`
- 启动心跳定时器（默认 60 秒）

#### Scenario: Agent 上报安装完成状态

**Given** 节点刚完成 Debian 安装
**When** Agent 首次启动（通过 preseed late_command 安装）
**Then** Agent 应当：
- 发布 MQTT 消息到 `node/{mac}/status`
- Payload: `{"status": "installed", "ip": "<当前IP>", "hostname": "<主机名>"}`
- 服务器接收后将节点状态更新为 "installed"

#### Scenario: Agent 定期心跳

**Given** Agent 已启动并运行
**When** 心跳定时器触发（每 60 秒）
**Then** Agent 应当：
- 收集节点信息（IP、hostname、uptime 等）
- 发布到 `node/{mac}/status`
- Payload: `{"status": "installed", "ip": "...", "hostname": "...", "uptime": 3600}`

---

## Related Capabilities
- `node-state-management`: 节点状态更新
- `node-installation`: Agent 安装和配置（preseed late_command）
- `rest-api`: 节点状态查询
