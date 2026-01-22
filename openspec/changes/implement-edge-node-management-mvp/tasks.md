# Tasks: 实现边缘节点管理 MVP

## Phase 1: 项目基础设施

- [ ] **1.1 初始化 Go 模块**
  - 创建 `go.mod`：`module github.com/lucheng0127/nodefoundry`
  - 设置 Go 版本为 1.22
  - 添加依赖：gin, bbolt, paho.mqtt.golang, dhcp, zap, testify

- [ ] **1.2 创建项目目录结构**
  - 创建 `cmd/nodefoundry/` 目录
  - 创建 `internal/` 子目录：api/, db/, dhcp/, ipxe/, model/, mqtt/, server/
  - 创建 `scripts/` 和 `config/` 目录

- [ ] **1.3 创建基础配置结构**
  - 定义 `Config` 结构体（HTTP/DHCP/MQTT 地址、数据库路径等）
  - 实现环境变量加载逻辑
  - 设置默认值（NF_HTTP_ADDR=:8080 等）

## Phase 2: 核心数据模型

- [ ] **2.1 实现 Node 模型**
  - 创建 `internal/model/node.go`
  - 定义 `Node` 结构体（MAC, IP, Status, timestamps 等）
  - 定义状态常量（STATE_DISCOVERED, STATE_INSTALLING, STATE_INSTALLED）
  - 实现 `IsValidStatus()` 和 `CanTransitionTo()` 方法
  - 编写单元测试

- [ ] **2.2 实现 NodeRepository 接口**
  - 创建 `internal/db/node_repository.go`
  - 定义 `NodeRepository` 接口（Save, FindByMAC, List, ListByStatus, UpdateStatus, Delete）
  - 编写接口文档注释

- [ ] **2.3 实现 BoltNodeRepository**
  - 创建 `internal/db/bolt_node_repository.go`
  - 实现 `NodeRepository` 接口的 bbolt 版本
  - 实现 `Save()` - JSON 序列化 + bbolt 事务
  - 实现 `FindByMAC()`, `List()`, `ListByStatus()`
  - 实现 `UpdateStatus()` - 带状态转换验证
  - 实现 `Delete()`
  - 编写单元测试（使用内存 bbolt）

## Phase 3: DHCP 节点发现

- [ ] **3.1 实现 DHCP 服务器基础结构**
  - 创建 `internal/dhcp/server.go`
  - 定义 `DHCPServer` 结构体
  - 实现 `Start()` 方法 - 监听 UDP :67
  - 实现基础 DHCP 包处理循环

- [ ] **3.2 实现 DHCP 包解析和节点创建**
  - 实现 MAC 地址提取和规范化
  - 实现 IP 地址提取
  - 调用 `repo.Save()` 创建/更新节点（状态=discovered）
  - 处理 DHCPOFFER 响应
  - 编写单元测试

## Phase 4: iPXE 脚本生成

- [ ] **4.1 实现 iPXE 脚本生成器**
  - 创建 `internal/ipxe/generator.go`
  - 定义 `Generator` 结构体（镜像 URL、服务器地址等）
  - 实现 `GenerateScript(mac)` - 生成基础 iPXE 引导脚本
  - 实现 `GenerateBootScript(mac)` - 生成完整启动脚本（含 kernel/initrd）
  - 编写单元测试（golden file 测试）

- [ ] **4.2 实现 preseed 配置生成**
  - 创建 `internal/ipxe/preseed.go`
  - 实现 `GeneratePreseed(mac)` - 生成 Debian preseed 配置
  - 使用 USTC 镜像源
  - 编写单元测试

## Phase 5: MQTT 客户端

- [ ] **5.1 实现 MQTT 客户端基础结构**
  - 创建 `internal/mqtt/client.go`
  - 定义 `Client` 结构体
  - 实现 `Start()` - 连接到 Mosquitto
  - 实现自动重连逻辑

- [ ] **5.2 实现状态消息订阅和处理**
  - 订阅 `node/+/status` 主题
  - 实现 `onStatusMessage()` 回调
  - 解析 JSON payload
  - 调用 `repo.UpdateStatus()` 更新节点状态
  - 更新 LastHeartbeat
  - 编写单元测试

- [ ] **5.3 实现指令发布**
  - 实现 `PublishCommand(mac, command)` 方法
  - 发布到 `node/{mac}/command` 主题
  - 错误处理和日志记录

## Phase 6: RESTful API

- [ ] **6.1 实现 Gin 路由和基础 Handler**
  - 创建 `internal/api/handler.go`
  - 定义 `Handler` 结构体（注入 repo, mqtt, ipxe）
  - 实现 `RegisterRoutes()` - 注册 `/api/v1` 路由
  - 实现 `/health` 健康检查端点

- [ ] **6.2 实现节点查询 API**
  - 实现 `ListNodes()` - GET `/api/v1/nodes`
  - 实现 `GetNode()` - GET `/api/v1/nodes/:mac`
  - 实现 404 错误处理
  - 编写集成测试

- [ ] **6.3 实现节点注册和更新 API**
  - 实现 `RegisterNode()` - POST `/api/v1/nodes`
  - MAC 地址格式验证
  - 实现 `UpdateNode()` - PATCH `/api/v1/nodes/:mac`
  - 处理 409 冲突
  - 编写集成测试

- [ ] **6.4 实现安装触发 API**
  - 实现 `TriggerInstall()` - POST `/api/v1/nodes/:mac/install`
  - 验证节点状态（must be discovered）
  - 更新状态为 installing
  - 调用 `mqtt.PublishCommand()`
  - 编写集成测试

- [ ] **6.5 实现 iPXE 和 preseed 端点**
  - 实现 `GetIPXEScript()` - GET `/ipxe/:mac/boot.ipxe`
  - 实现 `GetPreseed()` - GET `/preseed/:mac/preseed.cfg`
  - 返回 text/plain 内容
  - 处理 404
  - 编写集成测试

## Phase 7: 服务器启动和集成

- [ ] **7.1 实现 Server 结构**
  - 创建 `internal/server/server.go`
  - 定义 `Server` 结构体（组合所有服务）
  - 实现 `Start()` - 使用 errgroup 并发启动所有服务

- [ ] **7.2 实现数据库初始化**
  - 在 `Start()` 中初始化 bbolt
  - 创建 "nodes" bucket
  - 优雅关闭时关闭数据库

- [ ] **7.3 实现日志初始化**
  - 使用 zap 初始化全局 logger
  - 支持日志级别配置（NF_LOG_LEVEL）
  - 结构化日志（zap.String, zap.Error 等）

- [ ] **7.4 实现信号处理和优雅关闭**
  - 监听 SIGINT/SIGTERM
  - 使用 context 取消所有 goroutine
  - 关闭数据库和 MQTT 连接

## Phase 8: 主程序入口

- [ ] **8.1 实现 main.go**
  - 创建 `cmd/nodefoundry/main.go`
  - 解析配置（环境变量）
  - 初始化 logger
  - 初始化 server
  - 启动服务
  - 信号处理

## Phase 9: 配置和脚本

- [ ] **9.1 创建默认配置文件**
  - 创建 `config/default.yaml`（可选）
  - 文档化所有环境变量

- [ ] **9.2 创建 preseed 示例文件**
  - 创建 `scripts/preseed-example.cfg`
  - 文档说明如何自定义

- [ ] **9.3 创建部署脚本**
  - 创建 `scripts/deploy.sh`
  - 创建 `scripts/install-mosquitto.sh`

## Phase 10: 测试和文档

- [ ] **10.1 编写端到端测试**
  - 使用 testcontainers 启动 Mosquitto
  - 模拟 DHCP 请求
  - 模拟 MQTT 消息
  - 验证完整流程

- [ ] **10.2 创建 README.md**
  - 项目介绍
  - 快速开始指南
  - API 文档
  - 配置说明

- [ ] **10.3 测试覆盖率检查**
  - 运行 `go test -cover`
  - 确保核心模块覆盖率 ≥ 80%

---

## 并行化建议

以下任务可以并行开发：
- **Phase 2** (数据模型) 完成后，Phase 3、4、5 可以并行
- **Phase 6** (API) 可以在 Phase 2 完成后开始（使用 mock）
- **Phase 9** (配置脚本) 可以在任何时候进行

## 依赖关系

```
Phase 1 (基础设施)
    ↓
Phase 2 (数据模型) ← 依赖：Phase 1
    ↓
Phase 3 (DHCP) ┐
Phase 4 (iPXE) ├─ 并行，依赖：Phase 2
Phase 5 (MQTT) ┘
    ↓
Phase 6 (API) ← 依赖：Phase 2, 5
    ↓
Phase 7 (服务器) ← 依赖：Phase 3, 5, 6
    ↓
Phase 8 (main.go) ← 依赖：Phase 7
    ↓
Phase 9 (配置) ┐
Phase 10 (测试) ┘─ 并行
```
