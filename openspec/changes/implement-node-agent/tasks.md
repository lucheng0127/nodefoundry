## 1. Agent 核心实现

- [ ] 1.1 创建 `cmd/nodefoundry-agent/main.go` 主程序
- [ ] 1.2 实现网络接口 MAC 地址获取（`internal/agent/netif.go`）
- [ ] 1.3 实现 MQTT 连接和订阅（`internal/agent/mqtt.go`）
- [ ] 1.4 实现状态上报功能（`internal/agent/status.go`）
- [ ] 1.5 实现心跳定时器（30 秒间隔）

## 2. 命令处理框架

- [ ] 2.1 定义命令处理器接口（`internal/agent/command/handler.go`）
- [ ] 2.2 实现 RebootCommand 命令（`internal/agent/command/reboot.go`）
- [ ] 2.3 实现命令注册和分发逻辑（`internal/agent/dispatcher.go`）
- [ ] 2.4 添加命令执行错误处理和日志记录

## 3. 系统信息收集

- [ ] 3.1 实现 IP 地址获取（`internal/agent/info/ip.go`）
- [ ] 3.2 实现系统运行时长获取（`internal/agent/info/uptime.go`）
- [ ] 3.3 实现主机名获取（`internal/agent/info/hostname.go`）
- [ ] 3.4 添加信息收集失败时的降级处理

## 4. 配置管理

- [ ] 4.1 实现环境变量配置解析（`internal/agent/config/config.go`）
- [ ] 4.2 支持的环境变量：NF_MQTT_BROKER、NF_LOG_LEVEL、NF_MAC、NF_HEARTBEAT_INTERVAL
- [ ] 4.3 添加配置验证和默认值

## 5. 日志和错误处理

- [ ] 5.1 集成 zap 日志库
- [ ] 5.2 实现结构化日志记录
- [ ] 5.3 添加日志级别配置支持
- [ ] 5.4 实现优雅关闭机制（signal handling）

## 6. HTTP 端点（服务器端）

- [ ] 6.1 添加 `/agent/nodefoundry-agent` 端点（`internal/api/handler.go`）
- [ ] 6.2 添加 `/agent/nodefoundry-agent.service` 端点
- [ ] 6.3 准备 systemd service 文件模板

## 7. Agent 编译

- [ ] 7.1 添加 Makefile 编译目标（`make build-agent`）
- [ ] 7.2 配置交叉编译：linux/arm64 (RK3588)
- [ ] 7.3 添加编译产物输出目录管理

## 8. preseed 集成

- [ ] 8.1 更新 `internal/ipxe/preseed.go` 中的 late_command
- [ ] 8.2 添加 Agent 下载和安装命令
- [ ] 8.3 添加 Agent 服务启用命令
- [ ] 8.4 更新 `scripts/preseed-example.cfg` 参考文件

## 9. 测试

- [ ] 9.1 单元测试：MAC 地址获取
- [ ] 9.2 单元测试：系统信息收集
- [ ] 9.3 单元测试：命令处理器
- [ ] 9.4 集成测试：Agent MQTT 通信
- [ ] 9.5 测试：Agent 在真实节点上的安装和运行

## 10. 文档和部署

- [ ] 10.1 更新 README.md 添加 Agent 说明
- [ ] 10.2 编写 Agent 部署文档
- [ ] 10.3 添加 systemd 服务配置说明
- [ ] 10.4 更新 Taskfile.yml 添加 Agent 相关任务
