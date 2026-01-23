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

4. **静态网络配置方式**: Debian 12 使用哪种网络配置工具？
   - 当前决策：优先使用 preseed 内置的 netcfg（传统方式）
   - 备选方案：NetworkManager（更现代，但需要额外安装）
   - **决策**: 在 preseed 中检测环境，如果有 NetworkManager 则使用它，否则使用 netcfg

## Additional Design: 静态网络配置继承

### 背景

在 NodeFoundry Server 作为网关（路由器）的场景下：
- 集群内无外部 DHCP 服务
- NodeFoundry DHCP 服务器为节点分配 IP
- 安装后系统需要使用相同 IP（静态配置）

### Decision 7: 网络配置持久化

**选择**: DHCP 分配的网络配置持久化到节点记录

**实现**:
```go
// Node 模型扩展
type Node struct {
    MAC           string          `json:"mac"`
    IP            string          `json:"ip,omitempty"`
    Netmask       string          `json:"netmask,omitempty"`
    Gateway       string          `json:"gateway,omitempty"`
    DNS           string          `json:"dns,omitempty"`
    Hostname      string          `json:"hostname,omitempty"`
    Status        string          `json:"status"`
    LastHeartbeat time.Time       `json:"last_heartbeat,omitempty"`
    CreatedAt     time.Time       `json:"created_at"`
    UpdatedAt     time.Time       `json:"updated_at"`
    Extra         json.RawMessage `json:"extra,omitempty"`
}
```

### DHCP Handler 实现（基于 MAC 的固定 IP 分配）

```go
// DHCPServer DHCP 服务器
type DHCPServer struct {
    addr         string
    repo         db.NodeRepository
    ipPool       *IPPool  // IP 池管理器
    subnet       string   // 从环境变量加载：DHCP_SUBNET（如 255.255.255.0 或 255.255.0.0）
    gateway      string   // 从环境变量加载：DHCP_GATEWAY
    dns          string   // 从环境变量加载：DHCP_DNS
    proxyMode    bool     // 从环境变量加载：DHCP_PROXY_MODE
    logger       *zap.Logger
    server       *dhcpv4.Server
}

// handleDiscover 处理 DHCPDISCOVER
func (s *DHCPServer) handleDiscover(pkt *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
    mac := pkt.ClientHWAddr.String()

    // ProxyDHCP 模式：不分配 IP，仅返回引导选项
    if s.proxyMode {
        return s.handleProxyDiscover(pkt)
    }

    // 1. 从 bbolt 检查 Node.IP
    node, err := s.repo.FindByMAC(context.Background(), mac)
    if err != nil {
        // 节点不存在，创建新节点
        node = &model.Node{
            MAC:    mac,
            Status: model.STATE_DISCOVERED,
        }
    }

    // 2. 如果 Node.IP 为空，从 IP 池分配新 IP
    if node.IP == "" {
        ip, err := s.ipPool.Allocate(mac)
        if err != nil {
            return nil, fmt.Errorf("IP pool exhausted: %w", err)
        }
        node.IP = ip
        node.Netmask = s.subnet  // 使用配置的子网掩码
        node.Gateway = s.gateway
        node.DNS = s.dns

        // 持久化到 bbolt
        if err := s.repo.Save(context.Background(), node); err != nil {
            return nil, fmt.Errorf("failed to save node: %w", err)
        }
        s.logger.Info("Allocated new IP", zap.String("mac", mac), zap.String("ip", ip))
    }

    // 3. 构建 DHCPOFFER（使用固定 IP）
    resp, err := dhcpv4.New()
    if err != nil {
        return nil, err
    }

    resp.MessageType = dhcpv4.MessageTypeOffer
    resp.ServerIPAddr = net.ParseIP(s.gateway)  // siaddr
    resp.YourIPAddr = net.ParseIP(node.IP)      // 分配的 IP
    resp.SubnetMask = net.ParseIP(s.subnet)     // 子网掩码
    resp.Router = []net.IP{net.ParseIP(s.gateway)}
    resp.DNS = []net.IP{net.ParseIP(s.dns)}
    resp.IPAddressLeaseTime = 24 * time.Hour

    // Boot options
    resp.BootFileName = cfg.BootFilename
    resp.ServerHostName = s.gateway

    return resp, nil
}

// handleProxyDiscover 处理 ProxyDHCP 模式的 DHCPDISCOVER
// ProxyDHCP 模式：不分配 IP，仅返回引导选项
func (s *DHCPServer) handleProxyDiscover(pkt *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
    mac := pkt.ClientHWAddr.String()

    // ProxyDHCP: 仅创建/更新节点记录（不分配 IP）
    node, err := s.repo.FindByMAC(context.Background(), mac)
    if err != nil {
        // 节点不存在，创建新节点（无 IP）
        node = &model.Node{
            MAC:    mac,
            Status: model.STATE_DISCOVERED,
        }
        if err := s.repo.Save(context.Background(), node); err != nil {
            s.logger.Error("Failed to save node", zap.String("mac", mac), zap.Error(err))
        }
    }

    // 构建 ProxyDHCP DHCPOFFER（仅包含引导选项，不含 IP）
    resp, err := dhcpv4.New()
    if err != nil {
        return nil, err
    }

    resp.MessageType = dhcpv4.MessageTypeOffer
    resp.ServerIPAddr = net.ParseIP(s.gateway)

    // ProxyDHCP: 不包含 YourIP, SubnetMask, Router, DNS
    // 仅包含引导选项
    resp.BootFileName = cfg.BootFilename
    resp.ServerHostName = s.gateway

    s.logger.Debug("ProxyDHCP offer sent", zap.String("mac", mac))
    return resp, nil
}

// handleRequest 处理 DHCPREQUEST（续租）
func (s *DHCPServer) handleRequest(pkt *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
    // ProxyDHCP 模式：忽略 DHCPREQUEST
    if s.proxyMode {
        s.logger.Debug("ProxyDHCP: ignoring DHCPREQUEST")
        return nil, nil
    }

    mac := pkt.ClientHWAddr.String()

    // 获取节点，返回已分配的固定 IP
    node, err := s.repo.FindByMAC(context.Background(), mac)
    if err != nil {
        return nil, fmt.Errorf("node not found: %s", mac)
    }

    if node.IP == "" {
        return nil, fmt.Errorf("node has no IP assigned")
    }

    // 续租：延长租约时间，返回相同的 IP
    resp, _ := dhcpv4.New()
    resp.MessageType = dhcpv4.MessageTypeAck
    resp.YourIPAddr = net.ParseIP(node.IP)
    resp.SubnetMask = net.ParseIP(s.subnet)  // 使用配置的子网掩码
    resp.Router = []net.IP{net.ParseIP(s.gateway)}
    resp.DNS = []net.IP{net.ParseIP(s.dns)}
    resp.IPAddressLeaseTime = 24 * time.Hour

    return resp, nil
}
```

### IP 池管理（内存 + 持久化）

```go
// IPPool IP 池管理器
type IPPool struct {
    db        *bbolt.DB
    start     string  // DHCP_POOL_START
    end       string  // DHCP_POOL_END
    allocated map[string]string  // 内存 map: MAC -> IP
    mu        sync.RWMutex
}

const BUCKET_IP_ALLOCATIONS = "ip_allocations"

// NewIPPool 创建 IP 池，从 bbolt 恢复已分配的 IP
func NewIPPool(db *bbolt.DB, start, end string) (*IPPool, error) {
    pool := &IPPool{
        db:        db,
        start:     start,
        end:       end,
        allocated: make(map[string]string),
    }

    // 初始化 bucket
    if err := db.Update(func(tx *bbolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists([]byte(BUCKET_IP_ALLOCATIONS))
        return err
    }); err != nil {
        return nil, err
    }

    // 从 bbolt 恢复已分配的 IP
    if err := db.View(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(BUCKET_IP_ALLOCATIONS))
        if b == nil {
            return nil
        }
        return b.ForEach(func(k, v []byte) error {
            mac := string(k)
            ip := string(v)
            pool.allocated[mac] = ip
            return nil
        })
    }); err != nil {
        return nil, err
    }

    return pool, nil
}

// Allocate 分配新 IP 给 MAC
func (p *IPPool) Allocate(mac string) (string, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    // 检查是否已分配
    if ip, exists := p.allocated[mac]; exists {
        return ip, nil
    }

    // 从 IP 池中寻找可用 IP
    startIP := net.ParseIP(p.start)
    endIP := net.ParseIP(p.end)

    for ip := startIP; ipLessEqual(ip, endIP); inc(ip) {
        ipStr := ip.String()

        // 检查是否已被占用
        if !p.isAllocated(ipStr) {
            p.allocated[mac] = ipStr

            // 持久化到 bbolt
            if err := p.db.Update(func(tx *bbolt.Tx) error {
                b := tx.Bucket([]byte(BUCKET_IP_ALLOCATIONS))
                return b.Put([]byte(mac), []byte(ipStr))
            }); err != nil {
                delete(p.allocated, mac)
                return "", err
            }

            return ipStr, nil
        }
    }

    return "", fmt.Errorf("IP pool exhausted")
}

// isAllocated 检查 IP 是否已被分配
func (p *IPPool) isAllocated(ip string) bool {
    for _, allocated := range p.allocated {
        if allocated == ip {
            return true
        }
    }
    return false
}
```

### Decision 8: iPXE 脚本生成（传递网络参数）

```go
// GenerateByStatus 生成 iPXE 脚本，传递网络参数到 preseed
func (g *Generator) GenerateByStatus(ctx context.Context, mac string) (string, error) {
    node, err := g.repo.FindByMAC(ctx, mac)
    if err != nil {
        return "", err
    }

    switch node.Status {
    case model.STATE_INSTALLING:
        return g.generateInstallScript(mac, node), nil
    // ... 其他状态
    }
}

// generateInstallScript 生成安装脚本，传递 IP 参数
func (g *Generator) generateInstallScript(mac string, node *model.Node) string {
    // 从 bbolt 取 Node.IP
    assignedIP := node.IP

    return fmt.Sprintf(`#!ipxe
set node_url http://%s
set mac %s
set arch ${buildarch}

# 获取当前分配的 IP
set assigned-ip %s

# kernel 启动，传递网络参数到 preseed URL
kernel https://mirrors.ustc.edu.cn/debian/dists/bookworm/main/installer-${arch}/current/images/netboot/debian-installer/${arch}/linux \
    auto=true priority=critical \
    url=${node_url}/preseed/${mac}?ip=${assigned-ip}&netmask=%s&gateway=%s&dns=%s

initrd https://mirrors.ustc.edu.cn/debian/dists/bookworm/main/installer-${arch}/current/images/netboot/debian-installer/${arch}/initrd.gz
boot
`, g.serverAddr, mac, assignedIP, node.Netmask, node.Gateway, node.DNS)
}
```

### Decision 9: preseed 生成（处理查询参数）

```go
// PreseedGenerator preseed 生成器
type PreseedGenerator struct {
    repo    db.NodeRepository
    tpl     *template.Template  // preseed 模板
}

// Generate 生成 preseed 配置
func (g *PreseedGenerator) Generate(mac string, query url.Values) (string, error) {
    node, err := g.repo.FindByMAC(context.Background(), mac)
    if err != nil {
        return "", err
    }

    // 验证状态
    if node.Status != model.STATE_INSTALLING {
        return "", fmt.Errorf("node not in installing state: %s", node.Status)
    }

    // 从 query 参数获取网络配置
    ip := query.Get("ip")
    netmask := query.Get("netmask")
    gateway := query.Get("gateway")
    dns := query.Get("dns")

    // 验证 IP 与 Node.IP 一致
    if ip != node.IP {
        return "", fmt.Errorf("IP mismatch: query=%s, node=%s", ip, node.IP)
    }

    // 填充模板数据
    data := struct {
        MAC      string
        Hostname string
        IP       string
        Netmask  string
        Gateway  string
        DNS      string
    }{
        MAC:      mac,
        Hostname: fmt.Sprintf("node-%s", mac),
        IP:       ip,
        Netmask:  netmask,
        Gateway:  gateway,
        DNS:      dns,
    }

    var buf bytes.Buffer
    if err := g.tpl.Execute(&buf, data); err != nil {
        return "", err
    }

    return buf.String(), nil
}
```

**preseed 模板** (`templates/preseed.cfg`):
```bash
d-i debian-installer/locale string en_US
d-i keyboard-configuration/xkb-keymap select us

# 静态网络配置（从查询参数动态替换）
d-i netcfg/disable_autoconfig boolean true
d-i netcfg/disable_dhcp boolean true
d-i netcfg/get_ipaddress string {{.IP}}
d-i netcfg/get_netmask string {{.Netmask}}
d-i netcfg/get_gateway string {{.Gateway}}
d-i netcfg/get_nameservers string {{.DNS}}
d-i netcfg/confirm_static boolean true

d-i netcfg/get_hostname string {{.Hostname}}
d-i netcfg/get_domain string

# 镜像配置
d-i mirror/country string manual
d-i mirror/http/hostname string mirrors.ustc.edu.cn
d-i mirror/http/directory string /debian
d-i mirror/http/proxy string

# ... 其余配置

# Agent 安装（获取当前 DHCP 网卡的 MAC）
d-i preseed/late_command string \
  DHCP_iface=$(ip route | grep default | awk '{print $5}') && \
  DHCP_MAC=$(cat /sys/class/net/${DHCP_iface}/address | tr -d ':') && \
  in-target wget http://%{server}:8080/agent/nodefoundry-agent -O /usr/local/bin/nodefoundry-agent && \
  in-target chmod +x /usr/local/bin/nodefoundry-agent && \
  in-target wget http://%{server}:8080/agent/nodefoundry-agent.service -O /etc/systemd/system/nodefoundry-agent.service && \
  in-target sh -c 'echo "NF_MAC='${DHCP_MAC}'" > /etc/default/nodefoundry-agent' && \
  in-target sh -c 'echo "NF_MQTT_BROKER=%{server}:1883" >> /etc/default/nodefoundry-agent' && \
  in-target systemctl enable nodefoundry-agent.service
```

### 环境变量配置

```bash
# DHCP 配置
NF_DHCP_SUBNET=255.255.255.0          # 子网掩码（根据实际网络配置，如 255.255.0.0）
NF_DHCP_GATEWAY=192.168.1.1           # 网关（服务器 IP）
NF_DHCP_DNS=192.168.1.1                # DNS 服务器
NF_DHCP_POOL_START=192.168.1.100      # IP 池起始
NF_DHCP_POOL_END=192.168.1.200        # IP 池结束
NF_DHCP_PROXY_MODE=false              # ProxyDHCP 模式（true=仅引导选项，不分配 IP）
```

**ProxyDHCP 模式说明**：
- 当 `NF_DHCP_PROXY_MODE=true` 时，DHCP 服务器仅响应引导选项
- 不分配 IP 地址，不进行 IP 持久化
- 适用于存在外部 DHCP 服务器的环境

### 数据流程总结

**标准 DHCP 模式**（NF_DHCP_PROXY_MODE=false）:
```
1. DHCPDISCOVER (MAC: AABBCCDDEEFF)
   ↓
2. DHCP Handler: 检查 Node.IP，无则从 IP 池分配
   ↓
3. 持久化到 bbolt: Node.IP = "192.168.1.100", Netmask = "255.255.255.0"
   ↓
4. DHCPOFFER: YourIP = 192.168.1.100, Netmask（从配置）, Gateway, DNS
   ↓
5. DHCPACK: 确认分配
   ↓
6. iPXE 引导: chain http://server/boot/aabbccddeeff/boot.ipxe
   ↓
7. iPXE 脚本: set assigned-ip 192.168.1.100
   ↓
8. chain 到 preseed URL with query params:
   http://server/preseed/aabbccddeeff?ip=192.168.1.100&netmask=255.255.255.0&gateway=192.168.1.1&dns=192.168.1.1
   ↓
9. preseed 生成: 验证 Node.Status==installing, 验证 ip==Node.IP
   ↓
10. preseed 模板替换: 静态网络配置
   ↓
11. Debian 安装: 使用静态 IP
   ↓
12. 安装完成，系统重启后使用相同 IP（静态配置）
```

**ProxyDHCP 模式**（NF_DHCP_PROXY_MODE=true）:
```
1. DHCPDISCOVER (MAC: AABBCCDDEEFF)
   ↓
2. DHCP Handler: 仅创建节点记录（不分配 IP）
   ↓
3. ProxyDHCPOFFER: 仅包含引导选项（siaddr, bootfile）
   ↓
4. 主 DHCP 服务器: 分配 IP（如 192.168.1.50）
   ↓
5. DHCPACK: 主服务器确认 IP
   ↓
6. iPXE 引导: chain http://server/boot/aabbccddeeff/boot.ipxe
   ↓
7. iPXE 脚本: 获取当前 IP（通过 ${net0/ip}）
   ↓
8. chain 到 preseed URL:
   http://server/preseed/aabbccddeeff（无 query 参数，或使用 DHCP）
   ↓
9. preseed 生成: 无静态 IP，使用 DHCP
   ↓
10. Debian 安装: 使用 DHCP 获取 IP
```

### 风险与缓解

**风险 1**: 静态 IP 冲突
- **缓解**: DHCP IP 池管理 + bbolt 持久化确保不重复分配

**风险 2**: 服务重启后 IP 丢失
- **缓解**: IP 池从 bbolt 恢复（ip_allocations bucket + nodes bucket）

**风险 3**: preseed URL 参数被篡改
- **缓解**: 验证 query.ip == Node.IP，拒绝不匹配的请求

**风险 4**: 网络配置文件格式变化（Debian 版本）
- **缓解**: preseed 动态生成，适配不同版本

**风险 5**: ProxyDHCP 模式误配置导致 IP 分配混乱
- **缓解**:
  - ProxyDHCP 模式明确忽略 DHCPREQUEST
  - 不创建 ip_allocations 记录
  - 节点记录中 IP 字段保持为空
  - 日志中明确标记 "ProxyDHCP mode"
