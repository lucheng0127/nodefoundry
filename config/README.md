# NodeFoundry 配置说明

## 环境变量

NodeFoundry 通过环境变量进行配置，所有配置项都有默认值。

| 环境变量 | 默认值 | 说明 |
|---------|-------|------|
| `NF_HTTP_ADDR` | `:8080` | HTTP 服务监听地址 |
| `NF_DHCP_ADDR` | `:67` | DHCP 服务监听地址（需要 root 权限） |
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
