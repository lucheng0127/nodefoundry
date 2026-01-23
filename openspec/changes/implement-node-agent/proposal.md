# Change: 实现边缘节点 Agent

## Why

当前系统已完成服务器端的基础设施（DHCP 发现、iPXE 引导、MQTT 状态接收），但缺少在已安装节点上运行的 Agent。Agent 是节点与服务器通信的桥梁，负责状态上报、心跳维护和远程命令执行，是实现完整节点管理闭环的关键组件。

## What Changes

- **新增**: 实现边缘节点 Agent (`cmd/nodefoundry-agent/`)
  - 状态上报：通过 MQTT publish 到 `/node/{mac}/status`
  - 心跳机制：每 30 秒发送一次心跳
  - 命令订阅：订阅 `/node/{mac}/command`，支持 `reboot` 命令
  - systemd 服务集成：依赖网络就绪后自动启动
  - 目标平台：RK3588 (ARM64) + Debian 系统

- **新增**: DHCP IP 分配持久化
  - DHCP 分配的 IP 记录到 bbolt 节点信息中（IP、Netmask、Gateway、DNS）
  - 支持静态 IP 配置继承（网关场景）
  - preseed 生成静态网络配置（NetworkManager 或 /etc/network/interfaces）

- **扩展准备**: 命令框架设计支持未来扩展
  - 命令处理接口设计，便于添加新命令（如模型任务执行）
  - 命令响应机制（可选）：执行结果上报

## Impact

- **Affected specs**:
  - `mqtt-reporting` - MODIFIED: 更新 Agent 行为规范，添加命令订阅相关需求
  - `agent` - ADDED: 新增 Agent 能力规范
  - `node-discovery` - MODIFIED: 添加 DHCP IP 持久化需求
  - `node-installation` - MODIFIED: 添加静态网络配置需求

- **Affected code**:
  - `cmd/nodefoundry-agent/` - 新增 Agent 主程序
  - `internal/dhcp/server.go` - 记录分配的 IP 到节点信息
  - `internal/model/node.go` - 添加网络配置字段
  - `internal/ipxe/preseed.go` - 生成静态网络配置
  - `internal/api/handler.go` - 新增 Agent 文件下载端点
  - `scripts/preseed-example.cfg` - 更新 late_command 以集成 Agent 安装
