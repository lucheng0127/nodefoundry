# Design: 边缘节点管理 MVP

## Architecture Overview

```
                    ┌─────────────────────────────────────┐
                    │         NodeFoundry Server          │
                    │                                     │
                    │  ┌───────────────────────────────┐  │
                    │  │   HTTP Server (Gin)           │  │
                    │  │   /api/v1/nodes               │  │
                    │  │   /boot/:mac/boot.ipxe        │  │
                    │  │   /preseed/:mac/preseed.cfg   │  │
                    │  │   /agent/*                    │  │
                    │  └───────────┬───────────────────┘  │
                    │              │                       │
                    │  ┌───────────▼───────────────────┐  │
                    │  │      Handlers (api/)          │  │
                    │  │  - ListNodes                  │  │
                    │  │  - GetNode                    │  │
                    │  │  - UpdateNode (PUT)           │  │
                    │  │  - TriggerInstall (action)    │  │
                    │  │  - GetBootScript              │  │
                    │  │  - GetPreseed                 │  │
                    │  │  - GetAgentFiles              │  │
                    │  └───────────┬───────────────────┘  │
                    │              │                       │
                    │  ┌───────────▼───────────────────┐  │
                    │  │    NodeService               │  │
                    │  │  - business logic            │  │
                    │  │  - state transitions         │  │
                    │  └───────────┬───────────────────┘  │
                    │              │                       │
                    │  ┌───────────▼───────────────────┐  │
                    │  │    IPXEScriptGenerator       │  │
                    │  │  - GenerateByStatus(mac)     │  │
                    │  │  - discovered: wait loop     │  │
                    │  │  - installing: boot install  │  │
                    │  │  - installed: local boot     │  │
                    │  └───────────┬───────────────────┘  │
                    │              │                       │
┌───────────┐      │  ┌───────────▼───────────────────┐  │
│   DHCP    │      │  │    NodeRepository (interface) │  │
│  Client   │──────┼──┼  - Save(node)                │  │
│           │      │  │  - FindByMAC(mac)            │  │
└───────────┘      │  │  - List()                    │  │
                   │  │  - UpdateStatus(mac, status) │  │
                   │  └───────────┬───────────────────┘  │
                   │              │                       │
                   │  ┌───────────▼───────────────────┐  │
                   │  │   BoltNodeRepository          │  │
                   │  │   (bbolt implementation)      │  │
                   │  └───────────────────────────────┘  │
                   │                                     │
                   │  ┌───────────────────────────────┐  │
                   │  │   DHCPServer                  │  │
                   │  │   - Listen on :67             │  │
                   │  │   - Extract MAC from DHCPDISCOVER │
                   │  │   - Create node (discovered)   │  │
                   │  └───────────────────────────────┘  │
                   │                                     │
                   │  ┌───────────────────────────────┐  │
                   │  │   MQTTClient                  │  │
                   │  │   - Subscribe: node/+/status  │  │
                   │  │   - Receive status from agents│  │
                   │  └───────────────────────────────┘  │
                   └─────────────────────────────────────┘

┌───────────────────────────────────────────────────────┐
│                      Edge Node                         │
│                                                       │
│  ┌─────────────────────────────────────────────────┐ │
│  │  BIOS/UEFI → PXE → DHCP → TFTP → HTTP           │ │
│  │  (iPXE boot loop while status=discovered)       │ │
│  └─────────────────────────────────────────────────┘ │
│                         │                             │
│                         ▼                             │
│  ┌─────────────────────────────────────────────────┐ │
│  │  Debian Installer (preseed automation)          │ │
│  │  - late_command: install agent                  │ │
│  └─────────────────────────────────────────────────┘ │
│                         │                             │
│                         ▼                             │
│  ┌─────────────────────────────────────────────────┐ │
│  │  NodeFoundry Agent (systemd service)            │ │
│  │  - Publish: node/{mac}/status (heartbeat)       │ │
│  │  - No command subscription (MVP)                │ │
│  └─────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────┘

                        HTTP (iPXE/preseed/agent)
                               │
                    ┌──────────┴──────────┐
                    │   NodeFoundry       │
                    │   Server            │
                    └─────────────────────┘
                               │
                               ▼
┌───────────┐      ┌───────────┐      ┌───────────────┐
│   TFTP    │      │   HTTP    │      │     MQTT       │
│  Client   │──────│  Client   │──────│    Broker      │
│           │      │           │      │  (Mosquitto)   │
└───────────┘      └───────────┘      └───────────────┘
                           │                    ▲
                           │                    │
                    (iPXE/preseed)         (status上报)
                        /agent/*           (单向通信)
```

## Component Design

### 1. Node Model (`internal/model/node.go`)

```go
// Node 表示边缘节点
type Node struct {
    MAC           string          `json:"mac"`           // 小写、无分隔符
    IP            string          `json:"ip,omitempty"`
    Hostname      string          `json:"hostname,omitempty"`
    Status        string          `json:"status"`        // discovered/installing/installed
    LastHeartbeat time.Time       `json:"last_heartbeat,omitempty"`
    CreatedAt     time.Time       `json:"created_at"`
    UpdatedAt     time.Time       `json:"updated_at"`
    Extra         json.RawMessage `json:"extra,omitempty"`
}

// 状态常量
const (
    STATE_DISCOVERED = "discovered"
    STATE_INSTALLING = "installing"
    STATE_INSTALLED  = "installed"
)

// IsValidStatus 验证状态是否有效
func IsValidStatus(status string) bool {
    return status == STATE_DISCOVERED ||
           status == STATE_INSTALLING ||
           status == STATE_INSTALLED
}

// CanTransitionTo 检查状态转换是否合法
// MVP: discovered → installing → installed (单向，不支持回退或重装)
func (n *Node) CanTransitionTo(newStatus string) error {
    // 实现状态转换验证逻辑
}
```

### 2. NodeRepository Interface (`internal/db/node_repository.go`)

```go
// NodeRepository 定义节点存储接口
type NodeRepository interface {
    // Save 保存或更新节点
    Save(ctx context.Context, node *model.Node) error

    // FindByMAC 根据 MAC 地址查找节点
    FindByMAC(ctx context.Context, mac string) (*model.Node, error)

    // List 列出所有节点
    List(ctx context.Context) ([]*model.Node, error)

    // ListByStatus 按状态筛选节点
    ListByStatus(ctx context.Context, status string) ([]*model.Node, error)

    // UpdateStatus 更新节点状态（带转换验证）
    UpdateStatus(ctx context.Context, mac string, status string) error

    // Delete 删除节点
    Delete(ctx context.Context, mac string) error
}
```

### 3. BoltNodeRepository (`internal/db/bolt_node_repository.go`)

```go
// BoltNodeRepository bbolt 实现的 NodeRepository
type BoltNodeRepository struct {
    db     *bbolt.DB
    logger *zap.Logger
}

// Bucket 名称
const (
    BUCKET_NODES = "nodes"
)

// Save 实现
func (r *BoltNodeRepository) Save(ctx context.Context, node *model.Node) error {
    return r.db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(BUCKET_NODES))
        if b == nil {
            return fmt.Errorf("bucket not found")
        }

        data, err := json.Marshal(node)
        if err != nil {
            return err
        }

        return b.Put([]byte(node.MAC), data)
    })
}
```

### 4. DHCPServer (`internal/dhcp/server.go`)

```go
// DHCPServer DHCP 服务器
type DHCPServer struct {
    addr      string
    repo      db.NodeRepository
    logger    *zap.Logger
    server    *dhcpv4.Server
}

// Start 启动 DHCP 服务器
func (s *DHCPServer) Start(ctx context.Context) error {
    // 监听 UDP :67
    // 处理 DHCPDISCOVER
    // 提取 MAC 地址
    // 创建/更新节点（状态=discovered）
}

func (s *DHCPServer) handleDHCP(conn net.PacketConn) {
    // 解析 DHCP 包
    // 提取 MAC
    // 调用 repo.Save(node{MAC, Status: "discovered"})
}
```

### 5. IPXEScriptGenerator (`internal/ipxe/generator.go`)

```go
// Generator iPXE 脚本生成器
type Generator struct {
    serverAddr    string
    mirrorBaseURL string // USTC mirror
    repo          db.NodeRepository
}

// GenerateByStatus 根据节点状态生成 iPXE 脚本
func (g *Generator) GenerateByStatus(ctx context.Context, mac string) (string, error) {
    node, err := g.repo.FindByMAC(ctx, mac)
    if err != nil {
        return "", err
    }

    switch node.Status {
    case model.STATE_DISCOVERED:
        return g.generateWaitLoopScript(mac), nil
    case model.STATE_INSTALLING:
        return g.generateInstallScript(mac), nil
    case model.STATE_INSTALLED:
        return g.generateLocalBootScript(), nil
    default:
        return "", fmt.Errorf("unknown node status: %s", node.Status)
    }
}

// generateWaitLoopScript 生成等待循环脚本
func (g *Generator) generateWaitLoopScript(mac string) string {
    return fmt.Sprintf(`#!ipxe
set node_url http://%s
set mac %s

:loop
echo Node in discovered state, waiting for installation trigger...
sleep 90
chain ${node_url}/boot/${mac}/boot.ipxe || goto loop
`, g.serverAddr, mac)
}

// generateInstallScript 生成安装脚本
func (g *Generator) generateInstallScript(mac string) string {
    return fmt.Sprintf(`#!ipxe
set node_url http://%s
set mac %s
set arch ${buildarch}

kernel https://mirrors.ustc.edu.cn/debian/dists/bookworm/main/installer-${arch}/current/images/netboot/debian-installer/${arch}/linux
initrd https://mirrors.ustc.edu.cn/debian/dists/bookworm/main/installer-${arch}/current/images/netboot/debian-installer/${arch}/initrd.gz
imgargs linux auto=true priority=critical url=${node_url}/preseed/${mac}/preseed.cfg
boot
`, g.serverAddr, mac)
}

// generateLocalBootScript 生成本地启动脚本
func (g *Generator) generateLocalBootScript() string {
    return `#!ipxe
echo Booting from local disk...
exit
`
}
```

### 6. MQTTClient (`internal/mqtt/client.go`)

```go
// Client MQTT 客户端（仅接收状态）
type Client struct {
    broker   string
    client   mqtt.Client
    repo     db.NodeRepository
    logger   *zap.Logger
}

// Start 启动 MQTT 客户端
func (c *Client) Start(ctx context.Context) error {
    // 连接到 Mosquitto
    // 订阅 node/+/status
    // 处理消息
}

// onStatusMessage 处理状态消息
func (c *Client) onStatusMessage(client mqtt.Client, msg mqtt.Message) {
    // topic: node/aabbccddeeff/status
    // payload: {"status": "installed", "ip": "192.168.1.100", "hostname": "node-xxx"}
    // 更新节点状态
}
```

### 7. API Handlers (`internal/api/handler.go`)

```go
// Handler API 处理器
type Handler struct {
    repo      db.NodeRepository
    mqtt      *mqtt.Client
    ipxe      *ipxe.Generator
    logger    *zap.Logger
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.Engine) {
    v1 := r.Group("/api/v1")
    {
        nodes := v1.Group("/nodes")
        {
            nodes.GET("", h.ListNodes)
            nodes.GET("/:mac", h.GetNode)
            nodes.POST("", h.RegisterNode)     // 手动注册
            nodes.PUT("/:mac", h.UpdateNode)    // 触发安装（action: install）
        }
    }

    // iPXE 端点
    r.GET("/boot/:mac/boot.ipxe", h.GetBootScript)
    r.GET("/preseed/:mac/preseed.cfg", h.GetPreseed)

    // Agent 下载端点
    r.GET("/agent/nodefoundry-agent", h.GetAgentBinary)
    r.GET("/agent/nodefoundry-agent.service", h.GetAgentServiceFile)

    // 健康检查
    r.GET("/health", h.HealthCheck)
}

// UpdateNode 更新节点（支持 action: install）
func (h *Handler) UpdateNode(c *gin.Context) {
    mac := c.Param("mac")

    var req struct {
        Action string `json:"action"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    node, err := h.repo.FindByMAC(c.Request.Context(), mac)
    if err != nil {
        c.JSON(404, gin.H{"error": "node not found"})
        return
    }

    switch req.Action {
    case "install":
        // MVP: 仅支持 discovered → installing
        if node.Status != model.STATE_DISCOVERED {
            c.JSON(400, gin.H{"error": fmt.Sprintf("cannot install node with status '%s', only 'discovered' nodes can be installed", node.Status)})
            return
        }

        // 更新状态为 installing
        if err := h.repo.UpdateStatus(c.Request.Context(), mac, model.STATE_INSTALLING); err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }

        // 节点的 iPXE 循环将在下次重试时获取到安装脚本
        // 安装脚本将指向 /preseed/${mac}/preseed.cfg（系统动态生成）
        c.JSON(200, node)

    default:
        c.JSON(400, gin.H{"error": "unknown action"})
    }
}

// GetBootScript 获取 iPXE 引导脚本
func (h *Handler) GetBootScript(c *gin.Context) {
    mac := c.Param("mac")

    script, err := h.ipxe.GenerateByStatus(c.Request.Context(), mac)
    if err != nil {
        c.JSON(404, gin.H{"error": "node not found"})
        return
    }

    c.Header("Content-Type", "text/plain")
    c.String(200, script)
}
```

### 8. Agent Design (`cmd/nodefoundry-agent/`)

**Agent 运行在已安装的节点上，作为 systemd 服务：**

```go
// Agent 主程序
type Agent struct {
    mac        string
    broker     string
    client     mqtt.Client
    logger     *zap.Logger
}

// Start 启动 Agent
func (a *Agent) Start(ctx context.Context) error {
    // 1. 获取本机 MAC 地址
    // 2. 连接 MQTT Broker
    // 3. 发布初始状态到 node/{mac}/status
    // 4. 启动心跳定时器（60秒）
}

// publishStatus 发布状态
func (a *Agent) publishStatus() {
    // 获取当前 IP、hostname、uptime
    // 发布到 node/{mac}/status
}
```

**systemd service 文件：**

```ini
[Unit]
Description=NodeFoundry Agent
After=network.target mosquitto.service

[Service]
Type=simple
ExecStart=/usr/local/bin/nodefoundry-agent
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### 9. Server (`internal/server/server.go`)

```go
// Server 服务器
type Server struct {
    config     *Config
    httpServer *http.Server
    dhcpServer *dhcp.DHCPServer
    mqttClient *mqtt.Client
    repo       db.NodeRepository
    logger     *zap.Logger
}

// Config 配置
type Config struct {
    HTTPAddr      string
    DHCPAddr      string
    MQTTBroker    string
    MirrorURL     string
    DBPath        string
}

// Start 启动所有服务
func (s *Server) Start(ctx context.Context) error {
    // 1. 初始化 bbolt
    // 2. 初始化 repositories
    // 3. 启动 DHCP server
    // 4. 启动 MQTT client
    // 5. 启动 HTTP server

    group, ctx := errgroup.WithContext(ctx)

    group.Go(func() error {
        return s.dhcpServer.Start(ctx)
    })

    group.Go(func() error {
        return s.mqttClient.Start(ctx)
    })

    group.Go(func() error {
        return s.httpServer.ListenAndServe()
    })

    return group.Wait()
}
```

## Data Flow

### 节点发现流程
```
1. 新节点启动 → 发送 DHCPDISCOVER
2. DHCPServer 接收 → 提取 MAC
3. repo.Save(node{MAC, Status: "discovered"})
4. 响应 DHCPOFFER（含 TFTP 服务器地址）
5. 节点请求 iPXE（TFTP → HTTP /boot/<mac>/boot.ipxe）
6. 返回等待循环脚本（status=discovered）
7. 节点进入 iPXE 循环（sleep 90s + chain 回自己）
```

### 安装触发流程
```
1. Admin 调用 PUT /api/v1/nodes/<mac> {action: "install"}
2. Handler 检查节点状态（must be discovered）
3. repo.UpdateStatus(mac, "installing")
4. 节点的 iPXE 循环在下次重试时获取到安装脚本
5. iPXE 加载 kernel/initrd + preseed
6. 开始自动化安装
```

### 状态上报流程
```
1. Debian 安装完成 → preseed late_command 安装 agent
2. Agent 首次启动 → MQTT publish "node/<mac>/status"
3. Payload: {status: "installed", ip: "192.168.1.100", hostname: "node-xxx"}
4. MQTTClient.onStatusMessage() 接收
5. repo.UpdateStatus(mac, "installed")
6. 更新节点 IP、Hostname 和 LastHeartbeat
7. Agent 每 60 秒发送心跳
```

## Configuration

环境变量配置：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| NF_HTTP_ADDR | :8080 | HTTP 服务地址 |
| NF_DHCP_ADDR | :67 | DHCP 服务地址 |
| NF_MQTT_BROKER | localhost:1883 | MQTT Broker 地址 |
| NF_MIRROR_URL | mirrors.ustc.edu.cn | Debian 镜像源 |
| NF_DB_PATH | /var/lib/nodefoundry/nodes.db | bbolt 数据库路径 |
| NF_LOG_LEVEL | info | 日志级别 |

## Trade-offs

1. **bbolt vs PostgreSQL**
   - ✅ bbolt: 无需额外服务，轻量，适合边缘场景
   - ❌ bbolt: 单机存储，不支持分布式
   - **决策**: MVP 使用 bbolt，后续可迁移到 PostgreSQL

2. **无认证 API**
   - ✅ 简化 MVP 实现
   - ❌ 安全风险
   - **决策**: MVP 阶段接受，后续添加 Basic Auth 或 JWT

3. **iPXE 等待循环 vs MQTT 推送**
   - ✅ 等待循环：解决新节点无 agent 无法接收指令的问题
   - ❌ 90 秒延迟可能影响用户体验
   - **决策**: 使用 iPXE 循环，确保新节点能响应安装触发

4. **状态机单向转换（MVP 简化）**
   - ✅ 简单直接，降低复杂度
   - ❌ 不支持已安装节点重新安装
   - **决策**: MVP 仅支持 discovered → installing → installed

5. **MQTT 单向通信**
   - ✅ 简化 Agent 实现
   - ❌ 无法通过 MQTT 远程控制节点
   - **决策**: MVP 仅使用 MQTT 接收状态上报
