# node-state-management Specification

## Purpose
TBD - created by archiving change implement-edge-node-management-mvp. Update Purpose after archive.
## Requirements
### Requirement: 使用 bbolt 持久化节点状态

The system SHALL use bbolt embedded key-value database to persist node state.

Node storage SHALL:
1. Use bbolt as the database engine
2. Support transactions for data consistency
3. Provide NodeRepository interface for upper layers

#### Scenario: 保存新节点到数据库

**Given** 数据库已初始化
**When** 调用 `repo.Save(ctx, &model.Node{MAC: "aabbccddeeff", Status: "discovered"})`
**Then** 系统应当：
- 在 "nodes" bucket 中创建键为 "aabbccddeeff" 的记录
- 序列化节点为 JSON 存储
- 设置 CreatedAt 和 UpdatedAt 为当前时间
- 返回 nil error

#### Scenario: 更新已存在的节点

**Given** 数据库中已存在节点（MAC: AABBCCDDEEFF）
**When** 调用 `repo.Save(ctx, &model.Node{MAC: "aabbccddeeff", Status: "installing"})`
**Then** 系统应当：
- 更新现有节点的 Status 字段
- 更新 UpdatedAt 为当前时间
- 保持 CreatedAt 不变
- 返回 nil error

#### Scenario: 根据 MAC 查找节点

**Given** 数据库中存在节点（MAC: AABBCCDDEEFF）
**When** 调用 `repo.FindByMAC(ctx, "aabbccddeeff")`
**Then** 系统应当返回完整的节点信息

#### Scenario: 查找不存在的节点

**Given** 数据库为空
**When** 调用 `repo.FindByMAC(ctx, "000000000000")`
**Then** 系统应当返回 nil 和错误 "node not found"

#### Scenario: 列出所有节点

**Given** 数据库中存在 3 个节点
**When** 调用 `repo.List(ctx)`
**Then** 系统应当返回包含 3 个节点的切片，按 CreatedAt 降序排列

#### Scenario: 按状态筛选节点

**Given** 数据库中存在 2 个 "discovered" 状态和 1 个 "installed" 状态的节点
**When** 调用 `repo.ListByStatus(ctx, "discovered")`
**Then** 系统应当仅返回 2 个 "discovered" 状态的节点

### Requirement: 状态机控制节点生命周期

The system SHALL implement a strict state machine to control node lifecycle.

Node status transitions MUST follow: discovered → installing → installed

#### Scenario: 正常的状态转换

**Given** 节点状态为 "discovered"
**When** 调用 `repo.UpdateStatus(ctx, mac, "installing")`
**Then** 系统应当：
- 更新状态为 "installing"
- 更新 UpdatedAt 时间戳
- 返回 nil error

#### Scenario: 非法的状态转换（跳过状态）

**Given** 节点状态为 "discovered"
**When** 调用 `repo.UpdateStatus(ctx, mac, "installed")`
**Then** 系统应当返回错误 "invalid status transition: discovered -> installed"

#### Scenario: 非法的状态转换（回退）

**Given** 节点状态为 "installed"
**When** 调用 `repo.UpdateStatus(ctx, mac, "discovered")`
**Then** 系统应当返回错误 "invalid status transition: installed -> discovered"

#### Scenario: 无效的状态值

**Given** 节点状态为 "discovered"
**When** 调用 `repo.UpdateStatus(ctx, mac, "invalid")`
**Then** 系统应当返回错误 "invalid status: invalid"

### Requirement: 支持删除节点记录

The system SHALL support deletion of node records from the database.

#### Scenario: 删除已存在的节点

**Given** 数据库中存在节点（MAC: AABBCCDDEEFF）
**When** 调用 `repo.Delete(ctx, "aabbccddeeff")`
**Then** 系统应当：
- 从数据库中删除该节点记录
- 返回 nil error

#### Scenario: 删除不存在的节点

**Given** 数据库为空
**When** 调用 `repo.Delete(ctx, "000000000000")`
**Then** 系统应当返回 "node not found" 错误

### Requirement: 服务启动时自动初始化数据库

The system SHALL automatically initialize the database on service startup.

#### Scenario: 首次启动，数据库文件不存在

**Given** 数据库文件路径 `/var/lib/nodefoundry/nodes.db` 不存在
**When** 系统启动
**Then** 系统应当：
- 创建数据库文件
- 初始化 "nodes" bucket
- 记录启动日志

#### Scenario: 数据库文件已存在

**Given** 数据库文件已存在
**When** 系统启动
**Then** 系统应当：
- 打开现有数据库
- 验证 "nodes" bucket 存在
- 继续正常启动

---

