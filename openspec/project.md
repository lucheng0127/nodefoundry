# Project Context

## Purpose

NodeFoundry 是一个轻量级、边缘友好的 AI 节点管理平台，目标是为边缘计算场景提供节点发现、自动化系统安装（基于 iPXE + preseed）、状态管理和后续模型任务分发能力。

**MVP阶段核心目标：**

- 通过 DHCP 自动发现新节点（基于 MAC 地址）
- 支持基于 MAC 的动态 iPXE 引导和 preseed 自动化系统安装（使用 USTC 镜像源加速）
- 使用状态机管理节点生命周期：discovered → installing → installed
- 提供 RESTful API 供管理员查询/触发安装
- 安装完成后通过 MQTT 上报状态

**非功能性目标：**

- 极致轻量
- 高可扩展性：为后续模型管理、任务调度预留空间

## Tech Stack

- 语言：Go >= 1.22
- Web框架：Gin
- 数据库： bbolt（etcd-io/bbolt）
- MQTT Broker：Mosquitto
- MQTT Client：github.com/eclipse/paho.mqtt.golang
- DHCP Server：github.com/insomniacslk/dhcp
- TFTP Server：tftpd-hpa (系统软件包)
- iPXE：自定义构建
- 配置管理：Viper 或内置 flag + env
- 日志：zap
- 测试框架：Go 测试 + testify
- 构建工具：Taskfile
- 部署方式：systemd

## Project Conventions

- 模块名：github.com/lucheng0127/nodefoundry
- 主包：cmd/nodefoundry（可执行文件入口）
- 目录结构：
  ```
nodefoundry/
├── cmd/
│   └── nodefoundry/         # main 函数入口
├── internal/
│   ├── api/                 # Gin 路由、handler
│   ├── db/                  # bbolt 操作封装
│   ├── dhcp/                # DHCP server 实现
│   ├── ipxe/                # iPXE 脚本生成逻辑
│   ├── model/               # 核心领域模型（Node 等结构体）
│   ├── mqtt/                # MQTT 订阅/发布封装
│   └── server/              # HTTP + DHCP + MQTT 启动逻辑
├── pkg/                     # 可复用公共包（可选）
├── scripts/                 # 部署脚本、preseed 示例
├── config/                  # 默认配置文件
├── go.mod
├── Taskfile.yml             # 构建任务
└── README.md
  ```
- 包命名：小写、单数（model 而非 models）
- 文件命名：snake_case（如 node_repository.go）
- 接口命名：NodeRepository、IPXEScriptGenerator 等
- 常量：全大写 + 下划线（如 STATE_DISCOVERED）

### Code Style

- 严格遵循 Uber Go Style Guide + Effective Go
- 使用 goimports + gofumpt 格式化
- 错误处理：if err != nil { return err }（不使用 errors.Wrap，除非必要）
- 日志：使用 zap，结构化字段（zap.String("mac", mac)）
- 命名：
  - 变量/函数：camelCase
  - 类型/接口：PascalCase
  - 私有：小写开头
- 注释：每个导出类型/函数必须有 godoc 注释
- 最大行宽：120 字符
- 禁止使用 panic()（除 main），统一返回 error

### Architecture Patterns

- 分层架构：
  - Handler (api) → Service (业务逻辑) → Repository (db/dhcp/mqtt)
- 依赖倒置：使用接口解耦（NodeRepository 接口，bbolt 实现）
- 状态机：通过 Node.Status 字段 + 校验函数实现简单 FSM
- 单例模式：bbolt DB、MQTT Client 使用全局单例（或依赖注入）
- Goroutine 安全：bbolt 事务天然安全，MQTT 使用锁保护共享状态
- 配置驱动：所有路径、端口、镜像源从配置加载

### Testing Strategy

- 单元测试：覆盖所有 Repository 方法、Service 逻辑、IPXE 脚本生成
- Mock：使用 testify/mock 或 gomock Mock Repository、MQTT Client
- 测试覆盖率：核心模块 ≥ 80%
- 测试文件：放在同目录，命名 xxx_test.go
- golden 文件：IPXE 脚本生成使用 golden 测试

### Git Workflow

- 分支模型：GitHub Flow（适合 MVP）
  - main：始终可部署
  - 特性分支：feature/xxx、fix/xxx
  - PR 标题：feat: add node discovery via DHCP
- 提交信息：
  ```
feat: add dynamic ipxe script generation by mac
fix: correct mac address normalization
refactor: extract node repository interface
  ```

## Domain Context

- 核心领域：边缘节点（Edge Node）
- 聚合根：Node（以 MAC 地址为唯一标识）
- 值对象：状态（discovered/installing/installed）、时间戳等
- 边界上下文：
  - 节点发现（DHCP）
  - 节点安装（iPXE + preseed）
  - 状态同步（MQTT）

## Important Constraints

- 运行环境：Raspberry Pi 2B（ARMv7，1GB RAM，Raspbian Lite）
- 节点数量：≤ 100
- 网络：局域网，DHCP/TFTP/HTTP/MQTT 均需可达
- 安全性：MVP 阶段允许匿名 MQTT + 无认证 API，后续加 TLS + 认证
- preseed：使用 USTC 镜像源（mirrors.ustc.edu.cn/debian）
- iPXE：通过 HTTP chain 加载 preseed.cfg

## External Dependencies

```
require (
    github.com/gin-gonic/gin               v1.10.0
    github.com/etcd-io/bbolt               v1.3.10
    github.com/insomniacslk/dhcp           v0.0.0-...
    github.com/eclipse/paho.mqtt.golang    v1.4.3
    go.uber.org/zap                        v1.27.0
    github.com/stretchr/testify            v1.9.0  // 测试
)
```
