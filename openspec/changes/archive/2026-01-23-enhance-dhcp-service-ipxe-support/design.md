# Design: 增强 DHCP 服务以支持完整 iPXE 引导

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    DHCP Server Enhancement                      │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   IP Pool Manager                           │ │
│  │  - AllocateIP(mac) → ip                                   │ │
│  │  - ReleaseIP(ip)                                           │ │
│  │  - Track leases (in-memory map)                           │ │
│  └────────────────────────────────────────────────────────────┘ │
│                           │                                     │
│  ┌────────────────────────▼───────────────────────────────────┐ │
│  │                   DHCP Handler                             │ │
│  │  ┌───────────────────────────────────────────────────────┐ │ │
│  │  │ Standard Mode (default)                               │ │ │
│  │  │  - DHCPDISCOVER → DHCPOFFER (with IP + boot options)  │ │ │
│  │  │  - DHCPREQUEST  → DHCPACK (with IP + boot options)    │ │ │
│  │  └───────────────────────────────────────────────────────┘ │ │
│  │  ┌───────────────────────────────────────────────────────┐ │ │
│  │  │ ProxyDHCP Mode (optional)                             │ │ │
│  │  │  - DHCPDISCOVER → DHCPOFFER (ONLY boot options)       │ │ │
│  │  │  - DHCPREQUEST  → ignored (let main DHCP handle)      │ │ │
│  │  └───────────────────────────────────────────────────────┘ │ │
│  └────────────────────────────────────────────────────────────┘ │
│                           │                                     │
│                           ▼                                     │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Boot Options                             │ │
│  │  - siaddr (next-server): TFTP server IP                   │ │
│  │  - Option 66 (tftp-server-name): TFTP hostname           │ │
│  │  - Option 67 (bootfile-name): undionly.kpxe or ipxe.efi  │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      PXE/iPXE Client                            │
│                                                                  │
│  1. DHCPDISCOVER ────────────────────────┐                      │
│  2. DHCPOFFER (with boot options) ◄──────┘                      │
│  3. DHCPREQUEST (selected server)                                │
│  4. DHCPACK                                                        │
│  5. TFTP: GET undionly.kpxe ──────────────────┐                  │
│  6. HTTP: GET /boot/<mac>/boot.ipxe ◄─────────┘                  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Component Design

### 1. IP Pool Manager

```go
// IPManager IP 地址池管理器
type IPManager struct {
    start     net.IP
    end       net.IP
    netmask   net.IPMask
    gateway   net.IP
    dns       []net.IP
    leaseTime time.Duration

    // 租约管理：mac → ip
    leases map[string]*Lease

    // 反向索引：ip → mac
    allocated map[string]string

    mu sync.RWMutex
}

// Lease IP 租约
type Lease struct {
    MAC       string
    IP        net.IP
    ExpiresAt time.Time
}

// AllocateIP 为 MAC 地址分配 IP
func (m *IPManager) AllocateIP(mac string, requestedIP net.IP) (net.IP, error)

// ReleaseIP 释放 IP
func (m *IPManager) ReleaseIP(ip net.IP) error

// RenewLease 续租
func (m *IPManager) RenewLease(mac string) error
```

**配置示例：**

```yaml
# 环境变量配置
NF_DHCP_IP_POOL_START: "192.168.1.100"
NF_DHCP_IP_POOL_END:   "192.168.1.200"
NF_DHCP_NETMASK:       "255.255.255.0"
NF_DHCP_GATEWAY:       "192.168.1.1"
NF_DHCP_DNS:           "8.8.8.8,8.8.4.4"
NF_DHCP_LEASE_TIME:    "86400"  # 24小时（秒）
```

### 2. Enhanced DHCP Handler

```go
// DHCPServer 增强的 DHCP 服务器
type DHCPServer struct {
    addr      string
    repo      db.NodeRepository
    logger    *zap.Logger
    server    *server4.Server

    // 新增字段
    iface        string   // 绑定的网卡接口名（如 eth0），空则监听所有接口
    ipManager    *IPManager
    tftpServer   string   // TFTP 服务器 IP
    proxyMode    bool     // ProxyDHCP 模式
}

// buildResponse 构建增强的 DHCP 响应
func (s *DHCPServer) buildResponse(req *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
    resp, err := dhcpv4.NewReplyFromRequest(req)
    if err != nil {
        return nil, err
    }

    // 设置消息类型
    resp.UpdateOption(dhcpv4.OptMessageType(respType))

    // IP 分配（仅在标准模式）
    if !s.proxyMode {
        if req.MessageType() == dhcpv4.MessageTypeDiscover {
            // 分配新 IP
            ip, err := s.ipManager.AllocateIP(mac, req.RequestedIPAddress())
            resp.YourIPAddr = ip
        } else if req.MessageType() == dhcpv4.MessageTypeRequest {
            // 确认租约
            ip, _ := s.ipManager.GetLease(mac)
            resp.YourIPAddr = ip
        }

        // 设置网络参数
        resp.UpdateOption(dhcpv4.OptSubnetMask(s.ipManager.netmask))
        resp.UpdateOption(dhcpv4.OptRouter(s.ipManager.gateway))
        resp.UpdateOption(dhcpv4.OptDNS(s.ipManager.dns...))
        resp.UpdateOption(dhcpv4.OptIPAddressLeaseTime(uint32(s.ipManager.leaseTime.Seconds())))
    }

    // ===== 关键：设置 TFTP 引导选项 =====
    // siaddr (Next Server)
    if s.tftpServer != "" {
        resp.ServerIdentifier = net.ParseIP(s.tftpServer)
    }

    // Option 66 (TFTP Server Name)
    resp.UpdateOption(dhcpv4.OptTFTPServerName(s.tftpServer))

    // Option 67 (Bootfile Name) - 根据架构选择
    bootfile := s.getBootFile(req)
    resp.UpdateOption(dhcpv4.OptBootfileName(bootfile))

    return resp, nil
}

// getBootFile 根据客户端架构选择引导文件
func (s *DHCPServer) getBootFile(req *dhcpv4.DHCPv4) string {
    // 检查客户端系统架构
    // Option 93 (System Architecture)
    if archOpt := req.Options.Get(dhcpv4.OptionSystemArchitecture); archOpt != nil {
        arch := binary.BigEndian.Uint16(archOpt)

        // 0 = BIOS/IA32, 7 = EFI BC, 9 = EFI x86-64
        if arch == 0 {
            return "undionly.kpxe"  // BIOS
        } else if arch >= 7 {
            return "ipxe.efi"       // UEFI
        }
    }

    // 默认返回 iPXE 链式启动脚本
    return "undionly.kpxe"
}
```

### 3. ProxyDHCP Mode

```go
// handleDHCP ProxyDHCP 模式处理逻辑
func (s *DHCPServer) handleDHCP(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) {
    if s.proxyMode {
        // ProxyDHCP: 只响应 DISCOVER，忽略 REQUEST
        if msg.MessageType() == dhcpv4.MessageTypeDiscover {
            s.handleProxyDiscover(conn, peer, msg)
        }
        return
    }

    // 标准模式：处理所有消息
    s.handleStandard(conn, peer, msg)
}

// handleProxyDiscover ProxyDHCP 模式下的 DISCOVER 处理
func (s *DHCPServer) handleProxyDiscover(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) {
    // 构建 DHCPOFFER（仅包含引导选项，不含 IP）
    resp := s.buildProxyOffer(msg)

    // 发送响应（使用广播地址）
    resp.BroadcastAddress = net.IPv4bcast
    conn.WriteTo(resp.ToBytes(), &net.UDPAddr{IP: net.IPv4bcast, Port: 68})
}
```

## Data Flow

### Standard Mode (完整 DHCP 服务器)

```
Client                          NodeFoundry DHCP
  │                                   │
  ├─ DHCPDISCOVER ────────────────────>│
  │  (MAC: AA:BB:CC:DD:EE:FF)         │
  │                                   │
  │  创建/更新 Node{Status: discovered}│
  │  分配 IP: 192.168.1.100            │
  │                                   │
  ├───────────── DHCPOFFER ────────────┤
  │  - YourIP: 192.168.1.100           │
  │  - SubnetMask: 255.255.255.0       │
  │  - Router: 192.168.1.1             │
  │  - DNS: 8.8.8.8, 8.8.4.4           │
  │  - ServerIdentifier: 192.168.1.10  │
  │  - TFTP Server Name: 192.168.1.10  │
  │  - Bootfile Name: undionly.kpxe    │
  │                                   │
  ├─ DHCPREQUEST ─────────────────────>│
  │                                   │
  ├───────────── DHCPACK ──────────────┤
  │  (确认 IP 和引导选项)              │
```

### ProxyDHCP Mode (配合现有 DHCP)

```
Client               Existing DHCP    NodeFoundry ProxyDHCP
  │                         │                  │
  ├─ DHCPDISCOVER ──────────>│                  │
  │                         │                  │
  │<──────── DHCPOFFER ──────┤                  │
  │  (含 IP 分配)            │                  │
  │                         │                  │
  │<────────── DHCPOFFER ────┼──────────────────┤
  │  (仅含引导选项)          │                  │
  │  - siaddr: 192.168.1.10  │                  │
  │  - bootfile: undionly.kpxe                 │
  │                         │                  │
  ├─ DHCPREQUEST ──────────>│                  │
  │                         │                  │
  │<────────── DHCPACK ──────┤                  │
  │  (确认 IP)               │                  │
```

## Configuration

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `NF_DHCP_INTERFACE` | (无) | 绑定的网卡接口名，如 `eth0`、`ens33`。留空则监听所有接口（`0.0.0.0:67`） |
| `NF_DHCP_IP_POOL_START` | (无) | IP 池起始地址，如 `192.168.1.100` |
| `NF_DHCP_IP_POOL_END` | (无) | IP 池结束地址，如 `192.168.1.200` |
| `NF_DHCP_NETMASK` | `255.255.255.0` | 子网掩码 |
| `NF_DHCP_GATEWAY` | (无) | 网关地址 |
| `NF_DHCP_DNS` | `8.8.8.8,8.8.4.4` | DNS 服务器（逗号分隔） |
| `NF_DHCP_LEASE_TIME` | `86400` | 租约时间（秒），默认 24 小时 |
| `NF_DHCP_TFTP_SERVER` | (自动推断) | TFTP 服务器 IP 地址 |
| `NF_DHCP_PROXY_MODE` | `false` | 是否启用 ProxyDHCP 模式 |

### 网卡绑定

通过 `NF_DHCP_INTERFACE` 指定 DHCP 服务绑定的网卡接口：

```go
// listenAddr 根据 interface 配置确定监听地址
func listenAddr(iface string) (string, error) {
    if iface == "" {
        // 未指定网卡，监听所有接口
        return ":67", nil
    }

    // 查找指定网卡
    interfaces, err := net.Interfaces()
    if err != nil {
        return "", err
    }

    for _, i := range interfaces {
        if i.Name == iface {
            addrs, err := i.Addrs()
            if err != nil {
                return "", err
            }
            // 返回第一个 IPv4 地址
            for _, addr := range addrs {
                if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
                    if ipnet.IP.To4() != nil {
                        return ipnet.IP.String() + ":67", nil
                    }
                }
            }
        }
    }

    return "", fmt.Errorf("interface %s not found or no IPv4 address", iface)
}
```

### TFTP 服务器推断逻辑

如果未设置 `NF_DHCP_TFTP_SERVER`：

```go
func inferTFTPServer(httpAddr string) string {
    // 1. 从 HTTPAddr 推断
    //    :8080          → 使用本机 IP
    //    192.168.1.10:8080 → 192.168.1.10

    // 2. 如果是 :8080，扫描本机网络接口
    //    选择第一个非回环接口的 IPv4 地址

    // 3. 如果失败，返回空字符串（让客户端使用默认值）
    return ""
}
```

## Trade-offs

### 1. IP 池存储：内存 vs 数据库

| 方案 | 优点 | 缺点 | 决策 |
|------|------|------|------|
| 内存 (map) | 简单快速，适合 MVP | 重启丢失租约 | ✅ MVP 采用 |
| bbolt 持久化 | 租约持久化 | 增加复杂度 | 未来可扩展 |

### 2. ProxyDHCP vs 完整 DHCP 服务器

| 方案 | 优点 | 缺点 | 决策 |
|------|------|------|------|
| ProxyDHCP | 不干扰现有网络 | 需要配合主 DHCP | 可选模式 |
| 完整 DHCP | 独立运行 | 可能与现有 DHCP 冲突 | 默认模式 |

### 3. Bootfile 检测

| 方案 | 优点 | 缺点 | 决策 |
|------|------|------|------|
| Option 93 | 准确检测 UEFI/BIOS | 部分客户端不发送 | ✅ 采用 |
| User Class | 兼容性好 | 非标准 | 可选添加 |

## Implementation Notes

### 向后兼容性

```go
// 如果未配置 IP 池，保持原有行为
if s.ipManager == nil {
    // 简单回显客户端 IP
    resp.YourIPAddr = req.ClientIPAddr
} else {
    // 使用 IP 池分配
    ip, _ := s.ipManager.AllocateIP(mac, req.RequestedIPAddress())
    resp.YourIPAddr = ip
}
```

### 测试策略

1. **单元测试**
   - `IPManager.AllocateIP()` - 测试 IP 分配逻辑
   - `buildResponse()` - 验证 Option 66/67 正确性
   - `getBootFile()` - 测试架构检测

2. **集成测试**
   - 完整 DHCP DORA 流程
   - ProxyDHCP 模式
   - IP 池耗尽场景

3. **Golden File 测试**
   - DHCP 响应包格式对比
