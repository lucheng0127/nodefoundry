# Capability: Node Discovery

## ADDED Requirements

### Requirement: DHCP 自动发现局域网节点

The system SHALL automatically discover new edge nodes on the LAN through DHCP.

When a new device boots and sends a DHCPDISCOVER request, the system SHALL:
1. Listen on UDP port 67 for DHCP requests
2. Extract the client MAC address from the DHCP packet
3. Normalize MAC address to lowercase without separators (e.g., "001122334455")
4. Create or update a node record in the database with status "discovered"
5. Record the node's IP address (from DHCP Request)
6. Record the node's discovery timestamp

#### Scenario: 新节点首次启动并发送 DHCPDISCOVER

**Given** 系统正在运行且 DHCP 服务器已启动
**When** 新节点（MAC: AA:BB:CC:DD:EE:FF）发送 DHCPDISCOVER 请求
**Then** 系统应当：
- 成功解析 DHCP 包并提取 MAC 地址
- 在数据库中创建节点记录：
  - MAC: "aabbccddeeff"
  - Status: "discovered"
  - IP: <分配的 IP 地址>
  - CreatedAt: <当前时间>
  - UpdatedAt: <当前时间>
- 响应 DHCPOFFER，包含 TFTP 服务器地址

#### Scenario: 已知节点重新发送 DHCP 请求

**Given** 节点（MAC: AABBCCDDEEFF）已存在于数据库中，状态为 "installed"
**When** 该节点重新发送 DHCPDISCOVER 请求
**Then** 系统应当：
- 更新现有节点记录的 IP 地址
- 更新 UpdatedAt 时间戳
- 保持现有状态不变（不重置为 "discovered"）

#### Scenario: DHCP 服务器启动失败

**Given** 端口 67 已被其他进程占用
**When** 系统尝试启动 DHCP 服务器
**Then** 系统应当：
- 记录错误日志
- 返回明确的错误信息
- 不启动其他服务（快速失败）

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

## Related Capabilities
- `node-state-management`: 节点存储和状态管理
- `rest-api`: API 端点实现
