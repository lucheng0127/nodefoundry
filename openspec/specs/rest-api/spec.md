# rest-api Specification

## Purpose
TBD - created by archiving change implement-edge-node-management-mvp. Update Purpose after archive.
## Requirements
### Requirement: 提供 RESTful API 供管理员管理节点

The system SHALL provide RESTful API for administrators to manage nodes.

API specifications:
- Base path: `/api/v1`
- Use JSON for request/response format
- Use standard HTTP status codes
- Error response format: `{"error": "error message"}`

#### Scenario: 列出所有节点

**Given** 系统中存在 2 个节点
**When** 发送 GET 请求到 `/api/v1/nodes`
**Then** 系统应当返回 200 OK 和节点列表：
```json
[
  {
    "mac": "aabbccddeeff",
    "ip": "192.168.1.100",
    "status": "discovered",
    "created_at": "2026-01-22T10:00:00Z",
    "updated_at": "2026-01-22T10:00:00Z"
  },
  {
    "mac": "001122334455",
    "ip": "192.168.1.101",
    "status": "installed",
    "created_at": "2026-01-22T09:00:00Z",
    "updated_at": "2026-01-22T10:30:00Z",
    "last_heartbeat": "2026-01-22T10:30:00Z"
  }
]
```

#### Scenario: 根据 MAC 获取单个节点

**Given** 节点（MAC: AABBCCDDEEFF）存在
**When** 发送 GET 请求到 `/api/v1/nodes/aabbccddeeff`
**Then** 系统应当返回 200 OK 和节点信息

#### Scenario: 获取不存在的节点

**Given** 节点不存在
**When** 发送 GET 请求到 `/api/v1/nodes/000000000000`
**Then** 系统应当返回 404 Not Found：
```json
{
  "error": "node not found"
}
```

#### Scenario: 手动注册新节点

**Given** 节点不存在
**When** 发送 POST 请求到 `/api/v1/nodes`：
```json
{
  "mac": "aabbccddeeff",
  "ip": "192.168.1.100"
}
```
**Then** 系统应当返回 201 Created 和创建的节点信息，Location header 指向 `/api/v1/nodes/aabbccddeeff`

#### Scenario: 注册已存在的节点

**Given** 节点已存在
**When** 发送 POST 请求到 `/api/v1/nodes`，MAC 为 "aabbccddeeff"
**Then** 系统应当返回 409 Conflict：
```json
{
  "error": "node already exists"
}
```

#### Scenario: 注册节点时提供无效的 MAC 地址

**Given** 请求体包含无效的 MAC 地址
**When** 发送 POST 请求到 `/api/v1/nodes`：
```json
{
  "mac": "invalid-mac"
}
```
**Then** 系统应当返回 400 Bad Request：
```json
{
  "error": "invalid MAC address format"
}
```

#### Scenario: 更新节点信息（通用）

**Given** 节点（MAC: AABBCCDDEEFF）存在
**When** 发送 PATCH 请求到 `/api/v1/nodes/aabbccddeeff`：
```json
{
  "ip": "192.168.1.200"
}
```
**Then** 系统应当返回 200 OK 和更新后的节点信息

#### Scenario: 触发节点安装（仅 discovered 状态）

**Given** 节点（MAC: AABBCCDDEEFF）状态为 "discovered"
**When** 发送 PUT 请求到 `/api/v1/nodes/aabbccddeeff`：
```json
{
  "action": "install"
}
```
**Then** 系统应当：
- 将状态更新为 "installing"
- 返回 200 OK 和更新后的节点信息
- 节点的 iPXE 循环将在下次重试时获取到安装脚本
- 安装脚本将指向 `/preseed/aabbccddeeff/preseed.cfg`（系统动态生成）

#### Scenario: 触发安装时状态不正确（非 discovered）

**Given** 节点状态为 "installing" 或 "installed"
**When** 发送 PUT 请求到 `/api/v1/nodes/aabbccddeeff`，action 为 "install"
**Then** 系统应当返回 400 Bad Request：
```json
{
  "error": "cannot install node with status 'installing', only 'discovered' nodes can be installed"
}
```

### Requirement: 提供 iPXE 和 preseed 文件的 HTTP 端点

The system SHALL provide HTTP endpoints for iPXE boot scripts and preseed configuration files.

#### Scenario: 获取 iPXE 引导脚本（/boot/ 路径）

**Given** 节点（MAC: AABBCCDDEEFF）存在
**When** 发送 GET 请求到 `/boot/aabbccddeeff/boot.ipxe`
**Then** 系统应当：
- 根据节点状态返回相应的 iPXE 脚本
- Content-Type: `text/plain`
- 状态码 200 OK

#### Scenario: 获取不存在的节点的 iPXE 脚本

**Given** 节点不存在
**When** 发送 GET 请求到 `/boot/000000000000/boot.ipxe`
**Then** 系统应当返回 404 Not Found

#### Scenario: 获取 preseed 配置文件

**Given** 节点（MAC: AABBCCDDEEFF）存在
**When** 发送 GET 请求到 `/preseed/aabbccddeeff/preseed.cfg`
**Then** 系统应当返回 200 OK 和 preseed 配置内容，Content-Type: `text/plain`

### Requirement: 提供 Agent 下载端点

The system SHALL provide HTTP endpoints for agent binary and systemd service file download.

#### Scenario: 下载 agent 二进制文件

**Given** 系统正在运行
**When** 发送 GET 请求到 `/agent/nodefoundry-agent`
**Then** 系统应当返回 agent 二进制文件：
- Content-Type: `application/octet-stream`
- 状态码 200 OK

#### Scenario: 下载 systemd 服务文件

**Given** 系统正在运行
**When** 发送 GET 请求到 `/agent/nodefoundry-agent.service`
**Then** 系统应当返回 systemd 服务文件内容：
- Content-Type: `text/plain`
- 状态码 200 OK
- 文件内容为有效的 systemd service 配置

### Requirement: 提供健康检查端点

The system SHALL provide a health check endpoint for monitoring.

#### Scenario: 健康检查

**Given** 系统正在运行
**When** 发送 GET 请求到 `/health`
**Then** 系统应当返回 200 OK：
```json
{
  "status": "ok",
  "uptime": "5m30s"
}
```

---

