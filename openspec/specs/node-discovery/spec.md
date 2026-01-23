# node-discovery Specification

## Purpose
TBD - created by archiving change implement-edge-node-management-mvp. Update Purpose after archive.
## Requirements
### Requirement: DHCP 自动发现局域网节点

The system SHALL automatically discover new edge nodes on the LAN through DHCP.

The system SHALL support enhanced DHCP responses with complete network configuration when IP pool is configured.

#### Scenario: DHCP 响应包含完整网络参数

**Given** IP 池已配置
**When** 发送 DHCPOFFER 或 DHCPACK
**Then** 响应必须包含：
- YourIP (分配的 IP 地址)
- SubnetMask (子网掩码)
- Router (默认网关)
- DNS (DNS 服务器)
- IPAddressLeaseTime (租约时间)
- siaddr (TFTP 服务器)
- Option 66/67 (引导文件)

---

### Requirement: API 手动注册节点

The system SHALL support manual node registration through API in addition to DHCP auto-discovery.

#### Scenario: 管理员通过 API 手动注册节点

**Given** 系统正在运行
**When** 管理员发送 POST 请求到 `/api/v1/nodes`，请求体：
```json
{
  "mac": "001122334455",
  "ip": "192.168.1.100"
}
```
**Then** 系统应当：
- 创建节点记录，状态为 "discovered"
- 返回 201 Created 和创建的节点信息
- 如果 MAC 已存在，返回 409 Conflict

---

### Requirement: DHCP 响应必须包含 TFTP 引导选项

The system SHALL include proper TFTP boot options in all DHCP responses.

The DHCP DHCPOFFER and DHCPACK packets SHALL contain:
1. **siaddr field** (Next Server): TFTP server IP address
2. **Option 66** (TFTP Server Name): TFTP server hostname or IP
3. **Option 67** (Bootfile Name): iPXE boot filename

#### Scenario: DHCP Offer 包含 TFTP 引导选项

**Given** DHCP 服务器已配置 TFTP 服务器地址为 `192.168.1.10`
**When** 新节点发送 DHCPDISCOVER 请求
**Then** DHCPOFFER 响应必须包含：
- `siaddr`: `192.168.1.10`
- `Option 66`: `192.168.1.10`
- `Option 67`: `undionly.kpxe` (BIOS) 或 `ipxe.efi` (UEFI)

#### Scenario: 根据 Client System Architecture 选择引导文件

**Given** DHCP 请求包含 Option 93 (System Architecture)
**When** 客户端架构为以下值之一：
  - `0` (BIOS/IA32)
  - `7` (EFI BC)
  - `9` (EFI x86-64)
**Then** Option 67 必须返回相应引导文件：
  - 架构 `0` → `undionly.kpxe`
  - 架构 `7` 或 `9` → `ipxe.efi`
  - 未指定 → `undionly.kpxe` (默认)

#### Scenario: TFTP 服务器地址自动推断

**Given** 未配置 `NF_DHCP_TFTP_SERVER`
**When** HTTP 服务器监听在 `192.168.1.10:8080`
**Then** 系统应当：
- 自动使用 `192.168.1.10` 作为 TFTP 服务器地址
- 在所有 DHCP 响应中设置 `siaddr` 和 Option 66

---

### Requirement: IP 地址池管理

The system SHALL support configurable IP address pool management for DHCP clients.

When IP pool is configured, the system SHALL:
1. Allocate IPs from the configured pool range
2. Track active leases with expiration
3. Renew existing leases for known clients
4. Reject requests when pool is exhausted

#### Scenario: 从 IP 池分配地址

**Given** IP 池配置为：
  - Start: `192.168.1.100`
  - End: `192.168.1.200`
**When** 新节点（MAC: AABBCCDDEEFF）发送 DHCPDISCOVER
**Then** 系统应当：
- 从池中分配可用 IP（如 `192.168.1.100`）
- 创建租约记录：{MAC: "aabbccddeeff", IP: "192.168.1.100", ExpiresAt: <now + 24h>}
- 在 DHCPOFFER 中返回分配的 IP

#### Scenario: 已知客户端续租

**Given** 节点（MAC: AABBCCDDEEFF）持有 IP `192.168.1.100` 的有效租约
**When** 该节点发送 DHCPREQUEST 续租请求
**Then** 系统应当：
- 延长租约到期时间 24 小时
- 返回相同的 IP 地址 `192.168.1.100`
- 在 DHCPACK 中设置新的租约时间

#### Scenario: IP 池耗尽处理

**Given** IP 池中所有地址已分配（100/100 已使用）
**When** 新节点发送 DHCPDISCOVER 请求
**Then** 系统应当：
- 记录警告日志：`IP pool exhausted`
- 不发送 DHCPOFFER 响应
- 保持静默（让其他 DHCP 服务器响应）

#### Scenario: 向后兼容 - 未配置 IP 池

**Given** 未配置 IP 池参数
**When** 节点发送 DHCPDISCOVER 请求
**Then** 系统应当：
- 使用当前行为：回显客户端请求的 IP
- 不分配新地址
- 正常响应（保持向后兼容）

---

### Requirement: ProxyDHCP 模式支持

The system SHALL support optional ProxyDHCP mode for environments with existing DHCP servers.

In ProxyDHCP mode:
1. Only respond to DHCPDISCOVER messages
2. Do NOT assign IP addresses
3. Include ONLY boot options in DHCPOFFER
4. Ignore DHCPREQUEST and DHCPINFORM

#### Scenario: ProxyDHCP 模式启用

**Given** `NF_DHCP_PROXY_MODE=true`
**When** 新节点发送 DHCPDISCOVER 请求
**Then** 系统应当：
- 发送 DHCPOFFER（仅包含引导选项）
- **不包含**：YourIP、SubnetMask、Router、DNS
- **包含**：siaddr、Option 66、Option 67

#### Scenario: ProxyDHCP 模式忽略 DHCPREQUEST

**Given** ProxyDHCP 模式已启用
**When** 节点发送 DHCPREQUEST 请求
**Then** 系统应当：
- 忽略该请求（不响应）
- 让主 DHCP 服务器处理 IP 分配

#### Scenario: ProxyDHCP + 主 DHCP 协作流程

**Given** 网络中存在主 DHCP 服务器和 NodeFoundry ProxyDHCP
**When** 新节点启动并发送 DHCPDISCOVER
**Then** 流程应当是：
  ```
  Client → 主DHCP: DHCPDISCOVER
  Client ← 主DHCP: DHCPOFFER (含 IP: 192.168.1.50)
  Client ← ProxyDHCP: DHCPOFFER (仅含引导选项)
  Client → 主DHCP: DHCPREQUEST (选择主DHCP的offer)
  Client ← 主DHCP: DHCPACK (确认IP)
  ```

---

