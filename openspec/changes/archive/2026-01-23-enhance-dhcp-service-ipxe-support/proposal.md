# Proposal: 增强 DHCP 服务以支持完整 iPXE 引导

## Change ID
`enhance-dhcp-service-ipxe-support`

## Summary
完善当前最小化的 DHCP 实现，添加 IP 地址池管理、完整的 TFTP 引导选项（option 66/67）支持，以及可选的 ProxyDHCP 模式兼容，确保 iPXE 客户端能够正确获取引导文件并启动安装流程。

## Motivation

### 当前问题
现有 DHCP 实现（`internal/dhcp/server.go`）仅实现了最小化的节点发现功能：

1. **TFTP 引导选项缺失**
   - `getTFTPServerAddress()` 返回空字符串
   - DHCP Offer 中缺少 `siaddr` (next-server) 字段
   - `bootfile-name` 选项硬编码为 `undionly.kpxe`（仅适用于 BIOS）

2. **IP 分配过于简单**
   - 只回显客户端请求的 IP，不维护地址池
   - 无租约持久化，重启后可能导致 IP 冲突

3. **不支持 ProxyDHCP 模式**
   - 在存在现有 DHCP 服务器的环境中，无法作为 ProxyDHCP 运行
   - 限制了部署场景灵活性

### 为什么现在需要修复
- iPXE 依赖 DHCP Option 66 (next-server) 和 Option 67 (bootfile-name) 来定位 TFTP 服务器和引导文件
- 缺少这些选项会导致客户端 PXE 启动失败
- 完整的 IP 池管理是生产环境的基本要求

## Proposed Solution

### 1. DHCP Option 66/67 支持

在 `buildResponse()` 中正确设置引导选项：

| DHCP 字段 | Option Code | 用途 | 实现方式 |
|-----------|-------------|------|----------|
| `siaddr` | - | Next Server (TFTP IP) | `resp.ServerIdentifier` |
| Option 66 | 66 | TFTP Server Name | `OptTFTPServerName()` |
| Option 67 | 67 | Bootfile Name | `OptBootfileName()` |

### 2. IP 地址池管理

引入 IP 池配置和管理：

```go
type IP_POOL struct {
    Start    string   // 池起始 IP (如 192.168.1.100)
    End      string   // 池结束 IP (如 192.168.1.200)
    Netmask  string   // 子网掩码 (如 255.255.255.0)
    Gateway  string   // 网关地址 (如 192.168.1.1)
    DNS      []string // DNS 服务器
    LeaseTime uint32  // 租约时间（秒）
}

type Lease struct {
    MAC       string
    IP        string
    ExpiresAt time.Time
}
```

### 3. ProxyDHCP 模式（可选）

添加配置选项启用 ProxyDHCP：

```go
type DHCPServer struct {
    // ...
    proxyMode bool  // true = 仅响应 DHCPDISCOVER，不分配 IP
}
```

在 ProxyDHCP 模式下：
- 只响应 `DHCPDISCOVER`，发送 `DHCPOFFER`（仅包含引导选项）
- 不响应 `DHCPREQUEST`，不分配 IP 地址
- 由主 DHCP 服务器处理 IP 分配

## Scope

### 包含
- IP 地址池定义和管理（配置文件/环境变量）
- DHCP Option 66/67 正确填充
- 基本的租约管理（内存存储，可选持久化）
- ProxyDHCP 模式支持
- 引导文件名根据客户端架构选择（`undionly.kpxe` for BIOS, `ipxe.efi` for UEFI）

### 不包含
- 复杂的租约持久化（数据库存储）
- 动态 DNS 更新
- DHCP Failover/HA
- IPv6 支持

## Affected Capabilities

### 修改
- `node-discovery`: 增强 DHCP 服务器能力
  - ADDED: IP 地址池管理要求
  - MODIFIED: DHCP 响应必须包含 TFTP 选项
  - ADDED: ProxyDHCP 模式支持

### 新增（可选）
- `ipxe-boot`: iPXE 引导支持（可合并到 node-installation）

## Dependencies

### 外部依赖
- 无新增外部依赖

### 配置依赖
新增环境变量：
- `NF_DHCP_IP_POOL_START`: IP 池起始地址
- `NF_DHCP_IP_POOL_END`: IP 池结束地址
- `NF_DHCP_NETMASK`: 子网掩码
- `NF_DHCP_GATEWAY`: 网关地址
- `NF_DHCP_DNS`: DNS 服务器（逗号分隔）
- `NF_DHCP_TFTP_SERVER`: TFTP 服务器 IP
- `NF_DHCP_PROXY_MODE`: 是否启用 ProxyDHCP (true/false)

## Migration Plan

1. **Phase 1**: 添加 IP 池管理（保持向后兼容，默认行为不变）
2. **Phase 2**: 实现 Option 66/67 支持
3. **Phase 3**: 添加 ProxyDHCP 模式

### 向后兼容性
- 如果未配置 IP 池，保持当前行为（简单回显客户端 IP）
- ProxyDHCP 默认关闭

## Rollback Plan

回滚到 commit `7403dd6`（iptables 修改后的提交），删除新增的 IP 池相关代码。

## Related Issues
- 完善现有 MVP 的 DHCP 功能缺失
- 为生产环境部署做准备
