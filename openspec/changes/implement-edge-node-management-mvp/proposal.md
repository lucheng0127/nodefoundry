# Proposal: 实现边缘节点管理 MVP

## Change ID
`implement-edge-node-management-mvp`

## Summary
为 NodeFoundry 项目实现核心边缘节点管理功能，包括 DHCP 节点发现、iPXE 自动化安装、节点状态管理、RESTful API 和 MQTT 状态上报。

## Motivation
NodeFoundry 旨在为边缘计算场景提供轻量级的节点管理平台。在 MVP 阶段，需要实现：
- 自动发现局域网内新节点（通过 DHCP）
- 自动化操作系统安装（iPXE + preseed）
- 节点生命周期状态管理
- 管理员 API 接口
- 安装状态上报

当前项目为空，需要从零构建整个系统。

## Proposed Solution
采用分层架构 + 依赖倒置模式，使用纯 Go 技术栈实现：

1. **DHCP Server**: 基于 `github.com/insomniacslk/dhcp` 实现 DHCP 服务器，监听网络中的 DHCPDISCOVER 请求，提取 MAC 地址并记录为新节点
2. **iPXE Script Generator**: 动态生成 iPXE 引导脚本，支持基于 MAC 的差异化配置
3. **Node Repository**: 使用 bbolt 嵌入式数据库存储节点状态，提供 CRUD 接口
4. **RESTful API**: 基于 Gin 框架提供 `/api/v1/nodes` 端点
5. **MQTT Client**: 订阅节点安装状态上报，发布安装指令

## Scope
**包含：**
- DHCP 节点发现服务（端口 67）
- TFTP/HTTP 服务用于 iPXE 引导
- 动态 iPXE 脚本生成（支持 USTC 镜像源）
- bbolt 节点存储（状态机：discovered → installing → installed）
- RESTful API（GET/POST/PATCH `/api/v1/nodes`）
- MQTT 状态上报（节点 → 服务器）
- 配置管理（环境变量 + 默认值）

**不包含：**
- 用户认证（MVP 阶段允许匿名访问）
- TLS/HTTPS
- 节点分组管理
- 任务调度和模型分发（后续版本）

## Affected Capabilities
创建以下核心能力规格：
- `node-discovery`: DHCP 节点发现
- `node-installation`: iPXE + preseed 自动安装
- `node-state-management`: 节点状态机和存储
- `rest-api`: RESTful API 端点
- `mqtt-reporting`: MQTT 状态上报

## Dependencies
- 外部服务：Mosquitto MQTT Broker
- 系统软件：tftpd-hpa（TFTP 服务）
- 网络要求：DHCP/TFTP/HTTP/MQTT 端口可达

## Migration Plan
无需迁移，新项目从零开始。

## Rollback Plan
删除 `cmd/` 和 `internal/` 目录，回退到初始状态。

## Related Issues
无（初始实现）
