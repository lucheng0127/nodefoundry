## MODIFIED Requirements

### Requirement: IP 地址池管理

The system SHALL support configurable IP address pool management for DHCP clients.

When IP pool is configured, the system SHALL:
1. Allocate IPs from the configured pool range
2. Track active leases with expiration
3. Renew existing leases for known clients
4. Reject requests when pool is exhausted
5. **Persist allocated network configuration to node record**

#### Scenario: 从 IP 池分配地址并持久化

**Given** IP 池配置为：
  - Start: `192.168.1.100`
  - End: `192.168.1.200`
  - Netmask: `255.255.255.0`
  - Gateway: `192.168.1.1`
  - DNS: `192.168.1.1`
**When** 新节点（MAC: AABBCCDDEEFF）发送 DHCPDISCOVER
**Then** 系统应当：
- 从池中分配可用 IP（如 `192.168.1.100`）
- 创建租约记录：{MAC: "aabbccddeeff", IP: "192.168.1.100", ExpiresAt: <now + 24h>}
- 在 DHCPOFFER 中返回分配的 IP
- **将网络配置持久化到节点记录**：
  ```json
  {
    "mac": "aabbccddeeff",
    "ip": "192.168.1.100",
    "netmask": "255.255.255.0",
    "gateway": "192.168.1.1",
    "dns": "192.168.1.1",
    "status": "discovered"
  }
  ```
- 记录日志："Persisted network config for node aabbccddeeff: 192.168.1.100/24"

#### Scenario: 已知客户端续租

**Given** 节点（MAC: AABBCCDDEEFF）持有 IP `192.168.1.100` 的有效租约
**When** 该节点发送 DHCPREQUEST 续租请求
**Then** 系统应当：
- 延长租约到期时间 24 小时
- 返回相同的 IP 地址 `192.168.1.100`
- 在 DHCPACK 中设置新的租约时间
- **保持节点记录中的网络配置不变**（IP 地址固定）

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
- **不持久化网络配置**（因为没有分配）
- 正常响应（保持向后兼容）

### Requirement: 节点网络配置持久化

The system SHALL persist network configuration allocated via DHCP to node records in bbolt.

Node record SHALL include:
- ip: allocated IP address
- netmask: subnet mask
- gateway: default gateway
- dns: DNS server

#### Scenario: 服务重启后恢复网络配置

**Given** 节点（MAC: AABBCCDDEEFF）已分配 IP `192.168.1.100`
**When** NodeFoundry 服务重启
**Then** 系统应当：
- 从 bbolt 加载节点记录
- 恢复节点的网络配置信息
- 保持租约状态（如果尚未过期）
- 记录日志："Restored network config for node aabbccddeeff: 192.168.1.100/24"

#### Scenario: 节点网络配置查询

**Given** 节点（MAC: AABBCCDDEEFF）已分配 IP
**When** API 请求 GET `/api/v1/nodes/aabbccddeeff`
**Then** 系统应当返回包含网络配置的节点信息：
```json
{
  "mac": "aabbccddeeff",
  "ip": "192.168.1.100",
  "netmask": "255.255.255.0",
  "gateway": "192.168.1.1",
  "dns": "192.168.1.1",
  "status": "discovered",
  "created_at": "2024-01-23T10:00:00Z",
  "updated_at": "2024-01-23T10:00:00Z"
}
```

#### Scenario: 手动更新节点网络配置

**Given** 节点（MAC: AABBCCDDEEFF）已存在
**When** API 请求 PUT `/api/v1/nodes/aabbccddeeff`，请求体：
```json
{
  "ip": "192.168.1.101",
  "netmask": "255.255.255.0",
  "gateway": "192.168.1.1",
  "dns": "192.168.1.1"
}
```
**Then** 系统应当：
- 更新节点记录中的网络配置
- 更新 updated_at 时间戳
- 返回 200 OK 和更新后的节点信息
- 记录日志："Updated network config for node aabbccddeeff"
