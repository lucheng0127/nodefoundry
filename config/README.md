# NodeFoundry 配置说明

## 环境变量

NodeFoundry 通过环境变量进行配置，所有配置项都有默认值。

| 环境变量 | 默认值 | 说明 |
|---------|-------|------|
| `NF_HTTP_ADDR` | `:8080` | HTTP 服务监听地址 |
| `NF_DHCP_ADDR` | `:67` | DHCP 服务监听地址（需要 root 权限） |
| `NF_DHCP_INTERFACE` | (无) | DHCP 服务绑定的网卡接口（如 `eth0`、`ens33`） |
| `NF_DHCP_IP_POOL_START` | (无) | IP 池起始地址（如 `192.168.1.100`） |
| `NF_DHCP_IP_POOL_END` | (无) | IP 池结束地址（如 `192.168.1.200`） |
| `NF_DHCP_NETMASK` | `255.255.255.0` | 子网掩码 |
| `NF_DHCP_GATEWAY` | (无) | 网关地址（如 `192.168.1.1`） |
| `NF_DHCP_DNS` | `8.8.8.8,8.8.4.4` | DNS 服务器（逗号分隔） |
| `NF_DHCP_LEASE_TIME` | `86400` | 租约时间（秒），默认 24 小时 |
| `NF_DHCP_TFTP_SERVER` | (自动推断) | TFTP 服务器 IP 地址 |
| `NF_DHCP_PROXY_MODE` | `false` | 是否启用 ProxyDHCP 模式 |
| `NF_MQTT_BROKER` | `localhost:1883` | MQTT Broker 地址 |
| `NF_MIRROR_URL` | `mirrors.ustc.edu.cn` | Debian 镜像源地址 |
| `NF_DB_PATH` | `/var/lib/nodefoundry/nodes.db` | bbolt 数据库文件路径 |
| `NF_LOG_LEVEL` | `info` | 日志级别 (debug/info/warn/error) |
| `NF_SERVER_ADDR` | (自动推断) | iPXE/preseed 脚本中的服务器地址 |

### NF_SERVER_ADDR 说明

`NF_SERVER_ADDR` 用于生成 iPXE 和 preseed 脚本中的服务器 URL。如果不设置，系统会从 `NF_HTTP_ADDR` 自动推断。

- 如果 `NF_HTTP_ADDR` 为 `:8080`，默认推断为 `localhost:8080`
- 如果 `NF_HTTP_ADDR` 为 `0.0.0.0:8080`，默认推断为 `0.0.0.0:8080`
- 建议明确设置：`export NF_SERVER_ADDR=192.168.1.100:8080`

## DHCP 配置

### 标准模式（完整 DHCP 服务器）

在标准模式下，NodeFoundry 作为完整的 DHCP 服务器运行，为客户端分配 IP 地址并提供网络配置。

```bash
export NF_DHCP_ADDR=:67
export NF_DHCP_INTERFACE=eth0                    # 绑定到指定网卡（可选）
export NF_DHCP_IP_POOL_START=192.168.1.100       # IP 池起始地址
export NF_DHCP_IP_POOL_END=192.168.1.200         # IP 池结束地址
export NF_DHCP_NETMASK=255.255.255.0             # 子网掩码
export NF_DHCP_GATEWAY=192.168.1.1               # 网关
export NF_DHCP_DNS=8.8.8.8,8.8.4.4               # DNS 服务器
export NF_DHCP_LEASE_TIME=86400                  # 租约时间（秒）
export NF_DHCP_TFTP_SERVER=192.168.1.100         # TFTP 服务器 IP
```

**注意**：
- 如果网络中已有其他 DHCP 服务器，标准模式可能会产生冲突
- 确保配置的 IP 池不与其他 DHCP 服务器的地址范围重叠

### ProxyDHCP 模式（兼容现有 DHCP）

在网络中已有主 DHCP 服务器的情况下，可以启用 ProxyDHCP 模式。ProxyDHCP 不会分配 IP 地址，仅提供 TFTP 引导选项。

```bash
export NF_DHCP_ADDR=:67
export NF_DHCP_INTERFACE=eth0
export NF_DHCP_PROXY_MODE=true                   # 启用 ProxyDHCP
export NF_DHCP_TFTP_SERVER=192.168.1.100         # TFTP 服务器 IP
```

ProxyDHCP 模式下：
- 忽略 `NF_DHCP_IP_POOL_*` 相关配置（不分配 IP）
- 仅响应 `DHCPDISCOVER` 消息，发送引导选项
- 不响应 `DHCPREQUEST`，由主 DHCP 服务器处理 IP 分配
- 与现有 DHCP 服务器和平共存

### DHCP 网卡绑定

通过 `NF_DHCP_INTERFACE` 指定 DHCP 服务监听的网卡：

```bash
export NF_DHCP_INTERFACE=eth0    # 绑定到 eth0
```

- 留空则监听所有接口（`0.0.0.0:67`）
- 绑定指定网卡可以避免干扰其他网络
- 确保指定的网卡存在且已配置正确的 IP 地址

### TFTP 服务器推断

如果未设置 `NF_DHCP_TFTP_SERVER`：
- 从 `NF_SERVER_ADDR` 自动推断 IP 地址
- 例如：`NF_SERVER_ADDR=192.168.1.100:8080` → TFTP 为 `192.168.1.100`

### 向后兼容

如果未配置 IP 池（`NF_DHCP_IP_POOL_START` 和 `NF_DHCP_IP_POOL_END`），DHCP 服务器保持原有行为：
- 回显客户端请求的 IP 地址
- 不维护租约
- 仅用于节点发现

## 配置示例

### 开发环境

```bash
export NF_HTTP_ADDR=:8080
export NF_DHCP_ADDR=:67
export NF_MQTT_BROKER=localhost:1883
export NF_MIRROR_URL=mirrors.ustc.edu.cn
export NF_DB_PATH=./data/nodes.db
export NF_LOG_LEVEL=debug
export NF_SERVER_ADDR=localhost:8080
```

### 生产环境

```bash
export NF_HTTP_ADDR=:8080
export NF_DHCP_ADDR=:67
export NF_MQTT_BROKER=localhost:1883
export NF_MIRROR_URL=mirrors.ustc.edu.cn
export NF_DB_PATH=/var/lib/nodefoundry/nodes.db
export NF_LOG_LEVEL=info
export NF_SERVER_ADDR=192.168.1.100:8080
```

## systemd 配置

创建 `/etc/systemd/system/nodefoundry.service`:

```ini
[Unit]
Description=NodeFoundry Edge Node Management Server
After=network.target mosquitto.service
Requires=mosquitto.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/nodefoundry
Environment="NF_HTTP_ADDR=:8080"
Environment="NF_DHCP_ADDR=:67"
Environment="NF_MQTT_BROKER=localhost:1883"
Environment="NF_MIRROR_URL=mirrors.ustc.edu.cn"
Environment="NF_DB_PATH=/var/lib/nodefoundry/nodes.db"
Environment="NF_LOG_LEVEL=info"
Environment="NF_SERVER_ADDR=192.168.1.100:8080"
ExecStart=/usr/local/bin/nodefoundry
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable nodefoundry
sudo systemctl start nodefoundry
sudo systemctl status nodefoundry
```

## 端口说明

| 端口 | 协议 | 用途 |
|------|------|------|
| 67 | UDP | DHCP 服务 |
| 8080 | TCP | HTTP API 和文件服务 |

确保防火墙允许这些端口的流量：

```bash
# 安装 iptables-persistent（用于保存规则）
sudo apt-get install -y iptables-persistent

# 允许 DHCP (UDP 67)
sudo iptables -A INPUT -p udp --dport 67 -j ACCEPT

# 允许 HTTP API (TCP 8080)
sudo iptables -A INPUT -p tcp --dport 8080 -j ACCEPT

# 允许已建立的连接
sudo iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

# 允许本地回环
sudo iptables -A INPUT -i lo -j ACCEPT

# 保存规则
sudo netfilter-persistent save
# 或使用
sudo iptables-save > /etc/iptables/rules.v4

# 查看当前规则
sudo iptables -L -n -v
```
