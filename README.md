# NodeFoundry

边缘 AI 节点自动化管理系统 - 通过 DHCP、iPXE 和 MQTT 实现节点的自动发现、批量安装和状态监控。

## 功能特性

- **自动节点发现**: 通过 DHCP 自动发现新节点并注册
- **无人值守安装**: 使用 iPXE 和 Debian preseed 实现自动化系统安装
- **状态管理**: 节点状态跟踪（discovered → installing → installed）
- **边缘节点 Agent**: 已安装节点自动运行 Agent，上报状态和执行命令
- **静态网络配置**: 支持 DHCP 分配的 IP 持久化，安装后使用静态 IP
- **RESTful API**: 完整的节点管理 API
- **MQTT 通信**: 通过 MQTT 接收节点状态上报和心跳，支持远程命令
- **嵌入式数据库**: 使用 bbolt 进行轻量级数据持久化

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                    NodeFoundry Server                        │
│                                                              │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌─────────┐ │
│  │ DHCP      │  │ HTTP API │  │ iPXE     │  │ MQTT    │ │
│  │ Server    │  │ Handler  │  │ Generator│  │ Broker  │ │
│  └───────────┘  └───────────┘  └───────────┘  └─────────┘ │
│         │              │              │              │      │
│         └──────────────┴──────────────┴──────────────┘      │
│                        ↓                                    │
│                 ┌───────────────┐                           │
│                 │ NodeRepository│                           │
│                 │   (bbolt)     │                           │
│                 └───────────────┘                           │
└─────────────────────────────────────────────────────────────┘
                           ↑
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
   ┌─────┴─────┐     ┌─────┴─────┐     ┌─────┴─────┐
   │ 新节点     │     │ 安装中     │     │ 已安装     │
   │ PXE Boot  │     │ iPXE      │     │ Agent     │
   │ (DHCP)    │     │ (Preseed) │     │ (MQTT)    │
   └───────────┘     └───────────┘     └───────────┘
                                             │
                              ┌──────────────┼──────────────┐
                              │              │              │
                         状态上报          心跳维持        命令执行
                         (每30s)         (每30s)        (reboot等)
```

## 快速开始

### 前置要求

- Go 1.23+
- Mosquitto MQTT Broker
- Linux 系统（Debian/Ubuntu 推荐）
- Root 权限（DHCP 服务需要）

### 安装 Mosquitto

```bash
# 使用提供的安装脚本
sudo ./scripts/install-mosquitto.sh
```

或手动安装：

```bash
sudo apt-get update
sudo apt-get install -y mosquitto mosquitto-clients
sudo systemctl enable mosquitto
sudo systemctl start mosquitto
```

### 构建项目

```bash
# 克隆仓库
git clone https://github.com/lucheng0127/nodefoundry.git
cd nodefoundry

# 构建二进制文件
go build -o bin/nodefoundry ./cmd/nodefoundry
```

### 配置环境变量

```bash
# 基础配置
export NF_HTTP_ADDR=:8080
export NF_DHCP_ADDR=:67
export NF_MQTT_BROKER=localhost:1883
export NF_MIRROR_URL=mirrors.ustc.edu.cn
export NF_DB_PATH=./data/nodes.db
export NF_LOG_LEVEL=info
export NF_SERVER_ADDR=192.168.1.100:8080  # 替换为你的服务器 IP

# DHCP 高级配置（可选）
# 标准模式：完整 DHCP 服务器
# export NF_DHCP_INTERFACE=eth0                    # 绑定网卡（可选）
# export NF_DHCP_IP_POOL_START=192.168.1.100       # IP 池起始
# export NF_DHCP_IP_POOL_END=192.168.1.200         # IP 池结束
# export NF_DHCP_NETMASK=255.255.255.0
# export NF_DHCP_GATEWAY=192.168.1.1
# export NF_DHCP_DNS=8.8.8.8,8.8.4.4
# export NF_DHCP_LEASE_TIME=86400

# ProxyDHCP 模式：兼容现有 DHCP
# export NF_DHCP_PROXY_MODE=true
# export NF_DHCP_TFTP_SERVER=192.168.1.100
```

### 运行服务器

```bash
# 创建数据目录
mkdir -p data

# 运行服务器（需要 root 权限以绑定 DHCP 端口）
sudo ./bin/nodefoundry
```

### 部署到生产环境

```bash
# 使用部署脚本
sudo ./scripts/deploy.sh
```

## API 文档

### 健康检查

```bash
GET /health
```

响应：

```json
{
  "status": "ok",
  "uptime": "5m30s"
}
```

### 列出所有节点

```bash
GET /api/v1/nodes
```

响应：

```json
[
  {
    "mac": "aabbccddeeff",
    "ip": "192.168.1.100",
    "status": "discovered",
    "created_at": "2026-01-22T10:00:00Z",
    "updated_at": "2026-01-22T10:00:00Z"
  }
]
```

### 获取单个节点

```bash
GET /api/v1/nodes/:mac
```

### 注册新节点

```bash
POST /api/v1/nodes
Content-Type: application/json

{
  "mac": "aabbccddeeff",
  "ip": "192.168.1.100"
}
```

### 触发节点安装

```bash
PUT /api/v1/nodes/:mac
Content-Type: application/json

{
  "action": "install"
}
```

### 获取 iPXE 脚本

```bash
GET /boot/:mac/boot.ipxe
```

根据节点状态返回相应的 iPXE 脚本：
- `discovered`: 等待循环脚本
- `installing`: 安装脚本
- `installed`: 本地启动脚本

### 获取 Preseed 配置

```bash
GET /preseed/:mac/preseed.cfg
```

返回动态生成的 Debian preseed 自动安装配置。

支持查询参数传递网络配置（可选）：

```
GET /preseed/:mac/preseed.cfg?ip=192.168.1.100&netmask=255.255.255.0&gateway=192.168.1.1&dns=8.8.8.8
```

### 获取 Agent 二进制文件

```bash
GET /agent/nodefoundry-agent
```

返回 ARM64 架构的 Agent 二进制文件，用于在已安装节点上运行。

### 获取 Agent systemd 服务文件

```bash
GET /agent/nodefoundry-agent.service
```

返回 systemd 服务单元文件，用于配置 Agent 自动启动。

## 节点状态

节点有以下三种状态：

1. **discovered**: 节点通过 DHCP 发现，等待安装
2. **installing**: 安装已触发，节点正在安装系统
3. **installed**: 系统安装完成，agent 正常运行

状态转换规则：`discovered → installing → installed`（单向，MVP 不支持回退）

## 使用场景

### 场景 1: 自动发现和安装新节点

1. 将新节点连接到网络并启动 PXE boot
2. NodeFoundry DHCP 服务器自动发现节点，创建 `discovered` 状态记录
3. 节点进入 iPXE 等待循环
4. 管理员通过 API 触发安装：

```bash
curl -X PUT http://localhost:8080/api/v1/nodes/aabbccddeeff \
  -H "Content-Type: application/json" \
  -d '{"action": "install"}'
```

5. 节点的 iPXE 循环获取到新的安装脚本，开始安装 Debian
6. 安装过程中，preseed 的 late_command 自动下载并安装 NodeFoundry Agent
7. 安装完成后，系统重启，Agent 通过 systemd 自动启动
8. Agent 连接到 MQTT Broker，开始上报状态（每 30 秒）
9. 服务器通过 MQTT 订阅接收状态，更新节点为 `installed`

### Agent 功能

已安装节点上运行的 NodeFoundry Agent 提供以下功能：

- **状态上报**: 每 30 秒发布节点状态到 `node/<MAC>/status`
  - 包含：IP 地址、主机名、运行时长、时间戳
- **心跳维持**: 保持与 MQTT Broker 的连接
- **命令执行**: 订阅 `node/<MAC>/command`，支持远程命令
  - `reboot`: 重启节点
- **自动重启**: 通过 systemd 配置，崩溃后自动恢复

Agent 配置通过环境变量（`/etc/default/nodefoundry-agent`）：

| 环境变量 | 默认值 | 说明 |
|---------|-------|------|
| `NF_MAC` | (自动检测) | 节点 MAC 地址 |
| `NF_MQTT_BROKER` | `localhost:1883` | MQTT Broker 地址 |
| `NF_LOG_LEVEL` | `info` | 日志级别 |
| `NF_HEARTBEAT_INTERVAL` | `30` | 心跳间隔（秒） |

### 场景 2: 查询节点状态

```bash
# 列出所有节点
curl http://localhost:8080/api/v1/nodes

# 查询特定节点
curl http://localhost:8080/api/v1/nodes/aabbccddeeff
```

### 场景 3: 远程执行命令

通过 MQTT 向已安装节点发送命令：

```bash
# 发布重启命令
mosquitto_pub -h localhost -t "node/aabbccddeeff/command" -m '{"command":"reboot"}'
```

### 场景 4: 手动注册节点

如果节点不支持自动发现，可以手动注册：

```bash
curl -X POST http://localhost:8080/api/v1/nodes \
  -H "Content-Type: application/json" \
  -d '{"mac": "aabbccddeeff", "ip": "192.168.1.100"}'
```

## 配置说明

详细配置说明请参阅 [config/README.md](config/README.md)。

| 环境变量 | 默认值 | 说明 |
|---------|-------|------|
| `NF_HTTP_ADDR` | `:8080` | HTTP 服务地址 |
| `NF_DHCP_ADDR` | `:67` | DHCP 服务地址 |
| `NF_DHCP_INTERFACE` | (无) | DHCP 绑定的网卡接口 |
| `NF_DHCP_IP_POOL_START` | (无) | IP 池起始地址（标准模式） |
| `NF_DHCP_IP_POOL_END` | (无) | IP 池结束地址（标准模式） |
| `NF_DHCP_NETMASK` | `255.255.255.0` | 子网掩码 |
| `NF_DHCP_GATEWAY` | (无) | 网关地址 |
| `NF_DHCP_DNS` | `8.8.8.8,8.8.4.4` | DNS 服务器 |
| `NF_DHCP_LEASE_TIME` | `86400` | 租约时间（秒） |
| `NF_DHCP_TFTP_SERVER` | (自动推断) | TFTP 服务器 IP |
| `NF_DHCP_PROXY_MODE` | `false` | ProxyDHCP 模式 |
| `NF_MQTT_BROKER` | `localhost:1883` | MQTT Broker 地址 |
| `NF_MIRROR_URL` | `mirrors.ustc.edu.cn` | Debian 镜像源 |
| `NF_DB_PATH` | `/var/lib/nodefoundry/nodes.db` | 数据库路径 |
| `NF_LOG_LEVEL` | `info` | 日志级别 |
| `NF_SERVER_ADDR` | (自动推断) | 服务器地址 |

## 开发

### 目录结构

```
nodefoundry/
├── cmd/
│   ├── nodefoundry/          # 主程序入口
│   └── nodefoundry-agent/    # Agent 程序入口
├── internal/
│   ├── agent/                # Agent 核心功能
│   │   ├── command/          # 命令处理器
│   │   ├── config/           # Agent 配置
│   │   ├── info/             # 系统信息收集
│   │   ├── dispatcher.go     # 命令分发
│   │   ├── mqtt.go           # MQTT 客户端
│   │   └── netif.go          # 网络接口
│   ├── api/                  # HTTP API 处理器
│   ├── db/                   # 数据库层
│   ├── dhcp/                 # DHCP 服务器
│   │   ├── ip_pool.go        # IP 池管理
│   │   └── server.go         # DHCP 服务器
│   ├── ipxe/                 # iPXE 脚本生成
│   │   └── preseed.go        # Preseed 生成
│   ├── mqtt/                 # MQTT 客户端
│   ├── model/                # 数据模型
│   └── server/               # 服务器配置
├── scripts/                  # 部署和安装脚本
├── config/                   # 配置文件
└── openspec/                 # OpenSpec 规范
```

### 运行测试

```bash
go test ./...
```

### 构建 Agent

```bash
# 构建当前平台
make build-agent

# 构建 ARM64（RK3588 等边缘节点）
make build-agent-arm64
```

## 限制

当前 MVP 版本的限制：

1. **单向状态转换**: 不支持从 `installed` 状态回退或重新安装
2. **无认证**: API 未实现认证机制
3. **单机部署**: 使用 bbolt 嵌入式数据库，不支持分布式
4. **基础 DHCP**: DHCP 实现较简单，不支持复杂的网络配置
5. **Agent 平台**: Agent 目前仅支持 linux/arm64 (RK3588)
6. **无命令响应**: Agent 执行命令后不返回结果（仅日志记录）

## 静态网络配置

在标准 DHCP 模式下，NodeFoundry 支持将分配的 IP 持久化，使安装后的系统使用静态 IP：

### 工作原理

1. **IP 分配**: DHCP 服务器从 IP 池分配固定 IP 给节点
2. **持久化**: 网络配置（IP、Netmask、Gateway、DNS）保存到节点记录
3. **Preseed 生成**: iPXE 脚本将网络参数传递给 preseed URL
4. **静态配置**: 安装时使用静态网络配置

### 配置示例

```bash
# 启用 IP 池和网络配置
export NF_DHCP_IP_POOL_START=192.168.1.100
export NF_DHCP_IP_POOL_END=192.168.1.200
export NF_DHCP_NETMASK=255.255.255.0
export NF_DHCP_GATEWAY=192.168.1.1
export NF_DHCP_DNS=192.168.1.1
```

### 验证

查看节点记录中的网络配置：

```bash
curl http://localhost:8080/api/v1/nodes/aabbccddeeff
```

响应包含：
```json
{
  "mac": "aabbccddeeff",
  "ip": "192.168.1.100",
  "netmask": "255.255.255.0",
  "gateway": "192.168.1.1",
  "dns": "192.168.1.1"
}
```

### ProxyDHCP 模式

在 ProxyDHCP 模式下，不分配 IP，系统安装后使用 DHCP：

```bash
export NF_DHCP_PROXY_MODE=true
```



## DHCP 配置模式

NodeFoundry 支持两种 DHCP 运行模式，根据网络环境选择：

### 标准模式（完整 DHCP 服务器）

适用场景：独立网络、测试环境、无现有 DHCP 服务器

```bash
export NF_DHCP_INTERFACE=eth0                    # 绑定网卡（可选）
export NF_DHCP_IP_POOL_START=192.168.1.100       # IP 池起始
export NF_DHCP_IP_POOL_END=192.168.1.200         # IP 池结束
export NF_DHCP_NETMASK=255.255.255.0
export NF_DHCP_GATEWAY=192.168.1.1
export NF_DHCP_DNS=8.8.8.8,8.8.4.4
```

**注意**：确保网络中没有其他 DHCP 服务器，避免冲突。

### ProxyDHCP 模式（兼容现有 DHCP）

适用场景：生产环境、已有 DHCP 服务器、企业网络

```bash
export NF_DHCP_INTERFACE=eth0
export NF_DHCP_PROXY_MODE=true                   # 启用 ProxyDHCP
export NF_DHCP_TFTP_SERVER=192.168.1.100         # TFTP 服务器 IP
```

ProxyDHCP 模式下：
- 仅提供 PXE 引导选项，不分配 IP
- 与现有 DHCP 服务器和平共存
- 主 DHCP 处理 IP 分配，NodeFoundry 处理引导

## 故障排查

### 节点无法获取 IP

- 检查 DHCP 服务是否运行：`sudo systemctl status nodefoundry`
- 检查端口 67 是否被占用：`sudo netstat -ulnp | grep 67`
- 检查防火墙规则：`sudo iptables -L -n -v | grep 67`
- 如果是标准模式，确保网络中没有其他 DHCP 服务器

### 节点无法启动 PXE

- 检查 `NF_DHCP_TFTP_SERVER` 是否正确设置
- 确保独立的 TFTP 服务器已安装并运行
- 检查 TFTP 服务器根目录是否存在 `undionly.kpxe` 和 `ipxe.efi`
- 查看 DHCP 日志：`sudo journalctl -u nodefoundry -f`

### ProxyDHCP 不工作

- 确保主 DHCP 服务器允许 ProxyDHCP 响应
- 检查 `NF_DHCP_PROXY_MODE=true` 已设置
- 某些 DHCP 服务器可能需要配置以允许 ProxyDHCP

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！
