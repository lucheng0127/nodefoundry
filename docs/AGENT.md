# NodeFoundry Agent 部署文档

## 概述

NodeFoundry Agent 是运行在已安装边缘节点上的轻量级代理程序，负责：

- 通过 MQTT 上报节点状态和心跳
- 接收并执行远程命令（如 reboot）
- 与 NodeFoundry Server 保持通信

## 系统要求

- **操作系统**: Debian 12 (Bookworm) 或兼容系统
- **架构**: linux/arm64 (RK3588 等 ARM64 平台)
- **依赖**: systemd（服务管理）
- **网络**: 能访问 MQTT Broker

## 自动部署

Agent 通常通过 preseed 的 `late_command` 自动安装，无需手动干预。

### 安装过程

1. **下载二进制**: 从 NodeFoundry Server 下载 ARM64 二进制
2. **安装服务文件**: 下载并配置 systemd 服务单元
3. **生成配置**: 创建 `/etc/default/nodefoundry-agent` 环境变量文件
4. **启用服务**: 通过 systemd enable 自动启动

### 环境变量

Agent 通过以下环境变量配置：

| 变量 | 默认值 | 说明 |
|-----|-------|------|
| `NF_MAC` | (自动检测) | 节点 MAC 地址（小写，无分隔符） |
| `NF_MQTT_BROKER` | `localhost:1883` | MQTT Broker 地址 |
| `NF_LOG_LEVEL` | `info` | 日志级别（debug/info/warn/error） |
| `NF_HEARTBEAT_INTERVAL` | `30` | 心跳间隔（秒），最小 10 |

配置文件位置：`/etc/default/nodefoundry-agent`

```bash
NF_MAC=aabbccddeeff
NF_MQTT_BROKER=192.168.1.10:1883
NF_LOG_LEVEL=info
NF_HEARTBEAT_INTERVAL=30
```

## 手动部署

如果需要在已安装的系统上手动部署 Agent：

### 1. 下载 Agent

```bash
# 从 NodeFoundry Server 下载
wget http://<NF_SERVER_ADDR>:8080/agent/nodefoundry-agent -O /usr/local/bin/nodefoundry-agent

# 添加执行权限
chmod +x /usr/local/bin/nodefoundry-agent
```

### 2. 下载 systemd 服务文件

```bash
wget http://<NF_SERVER_ADDR>:8080/agent/nodefoundry-agent.service \
  -O /etc/systemd/system/nodefoundry-agent.service
```

服务文件内容：

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

### 3. 创建配置文件

```bash
# 获取当前网卡 MAC 地址
DHCP_iface=$(ip route | grep default | awk '{print $5}')
DHCP_MAC=$(cat /sys/class/net/${DHCP_iface}/address | tr -d ':')

# 创建配置文件
cat > /etc/default/nodefoundry-agent << EOF
NF_MAC=${DHCP_MAC}
NF_MQTT_BROKER=<NF_SERVER_ADDR>:1883
NF_LOG_LEVEL=info
NF_HEARTBEAT_INTERVAL=30
EOF
```

### 4. 启用并启动服务

```bash
# 重新加载 systemd 配置
systemctl daemon-reload

# 启用开机自启
systemctl enable nodefoundry-agent.service

# 启动服务
systemctl start nodefoundry-agent.service

# 检查服务状态
systemctl status nodefoundry-agent.service
```

## MQTT 通信

Agent 通过 MQTT 与 NodeFoundry Server 通信：

### 主题结构

- **状态上报**: `node/<MAC>/status`
- **命令接收**: `node/<MAC>/command`

### 状态消息

Agent 每 30 秒发布一次状态消息：

```json
{
  "status": "installed",
  "ip": "192.168.1.100",
  "hostname": "node-aabbccddeeff",
  "uptime": 3600,
  "timestamp": "2024-01-23T10:30:00Z"
}
```

### 命令消息

Server 可以发送命令到 Agent：

```json
{
  "command": "reboot",
  "args": {}
}
```

支持的命令：
- `reboot`: 重启节点（使用 `systemctl reboot`）

## 日志和调试

### 查看日志

```bash
# systemd 日志
journalctl -u nodefoundry-agent.service -f

# 查看最近 100 行
journalctl -u nodefoundry-agent.service -n 100
```

### 调试模式

修改配置文件启用调试日志：

```bash
# /etc/default/nodefoundry-agent
NF_LOG_LEVEL=debug
```

然后重启服务：

```bash
systemctl restart nodefoundry-agent.service
```

## 故障排查

### Agent 无法连接 MQTT Broker

**症状**: 日志显示 "failed to connect to MQTT broker"

**解决**:
1. 检查 `NF_MQTT_BROKER` 地址是否正确
2. 验证网络连接：`ping <MQTT_BROKER_IP>`
3. 检查 MQTT Broker 是否运行：`systemctl status mosquitto`
4. 检查防火墙规则

### MAC 地址获取错误

**症状**: 日志显示 "invalid MAC address" 或 "no valid network interface found"

**解决**:
1. 检查 `/etc/default/nodefoundry-agent` 中的 `NF_MAC` 值
2. 手动获取 MAC：`ip link show` 或 `cat /sys/class/net/<iface>/address`
3. MAC 地址应为小写、无分隔符格式（如：`aabbccddeeff`）

### Agent 服务未启动

**症状**: `systemctl status` 显示服务未运行

**解决**:
1. 检查二进制文件权限：`ls -l /usr/local/bin/nodefoundry-agent`
2. 检查服务文件语法：`systemctl daemon-reload`
3. 查看详细错误：`journalctl -u nodefoundry-agent.service -n 50`

## 升级 Agent

### 方法 1: 手动升级

```bash
# 停止服务
systemctl stop nodefoundry-agent.service

# 下载新版本
wget http://<NF_SERVER_ADDR>:8080/agent/nodefoundry-agent -O /usr/local/bin/nodefoundry-agent
chmod +x /usr/local/bin/nodefoundry-agent

# 启动服务
systemctl start nodefoundry-agent.service
```

### 方法 2: 通过命令升级（未来功能）

未来计划支持远程升级命令：

```bash
mosquitto_pub -h localhost -t "node/<MAC>/command" \
  -m '{"command":"update","args":{"version":"1.0.1"}}'
```

## 卸载 Agent

```bash
# 停止并禁用服务
systemctl stop nodefoundry-agent.service
systemctl disable nodefoundry-agent.service

# 删除文件
rm /usr/local/bin/nodefoundry-agent
rm /etc/systemd/system/nodefoundry-agent.service
rm /etc/default/nodefoundry-agent

# 重新加载 systemd
systemctl daemon-reload
```

## 性能特性

- **内存占用**: < 50MB
- **CPU 占用**: 空闲时 < 1%
- **网络开销**: 约 100 B/心跳（30 秒间隔）
- **启动时间**: < 2 秒

## 安全考虑

### 当前 MVP 限制

1. **无 TLS/认证**: MQTT 连接未加密
2. **无命令验证**: 任何可访问 MQTT 的人都可以发送命令
3. **明文传输**: 状态消息未加密

### 生产环境建议

1. 使用 TLS 加密 MQTT 连接
2. 启用 MQTT 用户认证
3. 使用 VPN 或防火墙限制 MQTT 访问
4. 定期更新 Agent 二进制文件

## 下一步

- [ ] 支持 Agent 版本管理和远程升级
- [ ] 添加命令执行结果上报
- [ ] 支持更多命令（如日志上传、配置更新）
- [ ] 添加本地配置文件支持
