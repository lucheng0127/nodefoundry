# NodeFoundry

边缘 AI 节点自动化管理系统 - 通过 DHCP、iPXE 和 MQTT 实现节点的自动发现、批量安装和状态监控。

## 功能特性

- **自动节点发现**: 通过 DHCP 自动发现新节点并注册
- **无人值守安装**: 使用 iPXE 和 Debian preseed 实现自动化系统安装
- **状态管理**: 节点状态跟踪（discovered → installing → installed）
- **RESTful API**: 完整的节点管理 API
- **MQTT 通信**: 通过 MQTT 接收节点状态上报和心跳
- **嵌入式数据库**: 使用 bbolt 进行轻量级数据持久化

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                    NodeFoundry Server                        │
│                                                              │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌─────────┐ │
│  │ DHCP      │  │ HTTP API │  │ iPXE     │  │ MQTT    │ │
│  │ Server    │  │ Handler  │  │ Generator│  │ Client  │ │
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
   └───────────┘     └───────────┘     └───────────┘
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
6. 安装完成后，agent 自动启动并通过 MQTT 上报状态
7. 服务器更新节点状态为 `installed`

### 场景 2: 查询节点状态

```bash
# 列出所有节点
curl http://localhost:8080/api/v1/nodes

# 查询特定节点
curl http://localhost:8080/api/v1/nodes/aabbccddeeff
```

### 场景 3: 手动注册节点

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
│   └── nodefoundry/          # 主程序入口
├── internal/
│   ├── api/                  # HTTP API 处理器
│   ├── db/                   # 数据库层
│   ├── dhcp/                 # DHCP 服务器
│   ├── ipxe/                 # iPXE 脚本生成
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

## 限制

当前 MVP 版本的限制：

1. **单向状态转换**: 不支持从 `installed` 状态回退或重新安装
2. **无认证**: API 未实现认证机制
3. **单机部署**: 使用 bbolt 嵌入式数据库，不支持分布式
4. **基础 DHCP**: DHCP 实现较简单，不支持复杂的网络配置

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
