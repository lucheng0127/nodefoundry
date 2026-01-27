# NodeFoundry Agent systemd 服务配置

## 概述

NodeFoundry Agent 通过 systemd 管理，实现自动启动、崩溃恢复和网络依赖管理。

## 服务文件

### 文件位置

`/etc/systemd/system/nodefoundry-agent.service`

### 完整配置

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

## 配置说明

### [Unit] 部分

| 配置项 | 值 | 说明 |
|-------|---|------|
| `Description` | `NodeFoundry Agent` | 服务描述 |
| `After` | `network-online.target` | 在网络就绪后启动 |
| `Wants` | `network-online.target` | 弱依赖网络，网络未就绪不阻塞启动 |

### [Service] 部分

| 配置项 | 值 | 说明 |
|-------|---|------|
| `Type` | `simple` | 服务类型（长期运行的前台进程） |
| `ExecStart` | `/usr/local/bin/nodefoundry-agent` | 启动命令 |
| `Restart` | `always` | 崩溃后自动重启 |
| `RestartSec` | `10` | 重启前等待 10 秒 |
| `EnvironmentFile` | `/etc/default/nodefoundry-agent` | 环境变量文件 |

### [Install] 部分

| 配置项 | 值 | 说明 |
|-------|---|------|
| `WantedBy` | `multi-user.target` | 在多用户模式下启动 |

## 环境变量配置

### 文件位置

`/etc/default/nodefoundry-agent`

### 示例配置

```bash
# MAC 地址（自动检测或手动设置）
NF_MAC=aabbccddeeff

# MQTT Broker 地址
NF_MQTT_BROKER=192.168.1.10:1883

# 日志级别
NF_LOG_LEVEL=info

# 心跳间隔（秒）
NF_HEARTBEAT_INTERVAL=30
```

## 服务管理命令

### 启用服务（开机自启）

```bash
systemctl enable nodefoundry-agent.service
```

### 禁用服务

```bash
systemctl disable nodefoundry-agent.service
```

### 启动服务

```bash
systemctl start nodefoundry-agent.service
```

### 停止服务

```bash
systemctl stop nodefoundry-agent.service
```

### 重启服务

```bash
systemctl restart nodefoundry-agent.service
```

### 重新加载配置

```bash
systemctl daemon-reload
```

### 查看服务状态

```bash
systemctl status nodefoundry-agent.service
```

输出示例：

```
● nodefoundry-agent.service - NodeFoundry Agent
     Loaded: loaded (/etc/systemd/system/nodefoundry-agent.service; enabled)
     Active: active (running) since Mon 2024-01-23 10:30:00 CST; 5h ago
   Main PID: 1234 (nodefoundry-agent)
      Tasks: 6 (limit: 1900)
     Memory: 12.3M (peak: 15.2M)
        CPU: 45ms
     CGroup: /system.slice/nodefoundry-agent.service
             └─1234 /usr/local/bin/nodefoundry-agent
```

## 日志管理

### 实时查看日志

```bash
journalctl -u nodefoundry-agent.service -f
```

### 查看最近日志

```bash
journalctl -u nodefoundry-agent.service -n 100
```

### 查看特定时间范围的日志

```bash
# 今天
journalctl -u nodefoundry-agent.service --since today

# 最近 1 小时
journalctl -u nodefoundry-agent.service --since "1 hour ago"

# 指定时间范围
journalctl -u nodefoundry-agent.service --since "2024-01-23 10:00:00" --until "2024-01-23 12:00:00"
```

### 过滤日志级别

```bash
# 只显示错误
journalctl -u nodefoundry-agent.service -p err

# 显示警告和错误
journalctl -u nodefoundry-agent.service -p warn..err
```

## 网络依赖管理

### network-online.target

服务配置 `After=network-online.target` 确保在网络就绪后才启动 Agent。

**注意**:
- `network-online.target` 不保证网络真正可用
- 只是表示网络管理服务已启动
- Agent 内部应处理连接失败和重连

### 网络不可用时的行为

1. **启动失败**: Agent 无法连接 MQTT Broker
2. **自动重连**: Agent 内置 MQTT 自动重连机制
3. **服务重启**: 如果 Agent 退出，systemd 会在 10 秒后重启

## 崩溃恢复

### 重启策略

`Restart=always` 配置确保以下情况都会自动重启：
- 程序崩溃（退出码非 0）
- 程序异常退出（信号杀死）
- 正常退出（退出码 0）

### 重启间隔

`RestartSec=10` 设置重启间隔为 10 秒，防止快速重启循环。

### 禁用自动重启

如果需要禁用自动重启（调试时）：

```bash
systemctl stop nodefoundry-agent.service
systemctl start nodefoundry-agent.service
# 或者临时覆盖
systemctl start nodefoundry-agent.service --property=Restart=no
```

## 资源限制

可选：添加资源限制防止资源占用过多

```ini
[Service]
Type=simple
ExecStart=/usr/local/bin/nodefoundry-agent
Restart=always
RestartSec=10
EnvironmentFile=/etc/default/nodefoundry-agent

# 资源限制
MemoryLimit=50M
CPUQuota=10%
```

## 安全加固

### 运行专用用户

创建专用用户运行 Agent：

```bash
# 创建用户
useradd -r -s /bin/false nodefoundry

# 修改服务文件
[Service]
User=nodefoundry
Group=nodefoundry
```

### 文件权限

```bash
# 二进制文件
chown root:root /usr/local/bin/nodefoundry-agent
chmod 755 /usr/local/bin/nodefoundry-agent

# 配置文件
chown root:root /etc/default/nodefoundry-agent
chmod 644 /etc/default/nodefoundry-agent

# 服务文件
chown root:root /etc/systemd/system/nodefoundry-agent.service
chmod 644 /etc/systemd/system/nodefoundry-agent.service
```

## 故障排查

### 服务无法启动

1. **检查配置文件**:
   ```bash
   systemd-analyze verify nodefoundry-agent.service
   ```

2. **查看详细错误**:
   ```bash
   journalctl -u nodefoundry-agent.service -n 50 --no-pager
   ```

3. **手动测试**:
   ```bash
   /usr/local/bin/nodefoundry-agent
   ```

### 服务频繁重启

1. **查看重启计数**:
   ```bash
   systemctl show nodefoundry-agent.service -p NRestarts
   ```

2. **检查日志**:
   ```bash
   journalctl -u nodefoundry-agent.service --boot
   ```

3. **临时禁用自动重启**:
   ```ini
   [Service]
   Restart=no
   ```

### 环境变量未生效

1. **验证文件存在**:
   ```bash
   cat /etc/default/nodefoundry-agent
   ```

2. **检查权限**:
   ```bash
   ls -l /etc/default/nodefoundry-agent
   ```

3. **重新加载**:
   ```bash
   systemctl daemon-reload
   systemctl restart nodefoundry-agent.service
   ```

## 自定义配置

### 修改启动参数

如需添加命令行参数：

```ini
[Service]
ExecStart=/usr/local/bin/nodefoundry-agent --debug --config=/etc/nodefoundry/config.yaml
```

### 添加依赖服务

如果 Agent 依赖其他服务：

```ini
[Unit]
After=network-online.target mosquitto.service
Wants=mosquitto.service
```

### 自定义重启策略

```ini
[Service]
# 仅在非正常退出时重启
Restart=on-failure

# 限制重启次数
StartLimitInterval=300
StartLimitBurst=5
```

## 最佳实践

1. **使用 EnvironmentFile**: 避免硬编码配置
2. **启用日志轮转**: 配置 journald 日志保留策略
3. **监控服务状态**: 使用监控工具（如 Prometheus）监控服务健康
4. **定期备份配置**: 备份 `/etc/default/nodefoundry-agent`
5. **版本管理**: 在 Agent 二进制文件名中包含版本号
