# Tasks: 增强 DHCP 服务以支持完整 iPXE 引导

## Phase 1: IP 地址池管理

- [ ] **1.1 实现 IPManager 结构**
  - 创建 `internal/dhcp/ip_pool.go`
  - 定义 `IPManager` 结构体（start, end, netmask, gateway, dns, leases）
  - 定义 `Lease` 结构体（MAC, IP, ExpiresAt）

- [ ] **1.2 实现 IP 分配算法**
  - 实现 `AllocateIP(mac, requestedIP) → ip`
  - 实现 `ReleaseIP(ip)`
  - 实现 `RenewLease(mac)`
  - 实现 `GetLease(mac) → ip`
  - 使用 `sync.RWMutex` 保护并发访问

- [ ] **1.3 添加 IP 池配置加载**
  - 在 `Config` 结构体中添加 IP 池字段
  - 从环境变量加载配置：
    - `NF_DHCP_IP_POOL_START`
    - `NF_DHCP_IP_POOL_END`
    - `NF_DHCP_NETMASK`
    - `NF_DHCP_GATEWAY`
    - `NF_DHCP_DNS`
    - `NF_DHCP_LEASE_TIME`

- [ ] **1.4 编写 IP 池单元测试**
  - 测试 IP 分配逻辑
  - 测试 IP 释放
  - 测试池耗尽场景
  - 测试并发分配

## Phase 2: TFTP 引导选项支持

- [ ] **2.1 实现 TFTP 服务器地址推断**
  - 实现 `inferTFTPServer(httpAddr) → ip`
  - 从 HTTPAddr 解析 IP
  - 支持本机接口扫描（当 HTTPAddr 为 `:port` 时）
  - 添加配置字段 `tftpServer`

- [ ] **2.2 实现引导文件名选择**
  - 实现 `getBootFile(dhcpRequest) → filename`
  - 解析 Option 93 (System Architecture)
  - 返回 `undionly.kpxe` for BIOS (arch=0)
  - 返回 `ipxe.efi` for UEFI (arch>=7)

- [ ] **2.3 增强 buildResponse() 方法**
  - 设置 `resp.ServerIdentifier` (siaddr)
  - 添加 `OptTFTPServerName()` (Option 66)
  - 添加 `OptBootfileName()` (Option 67)
  - 在标准模式下添加网络参数（SubnetMask, Router, DNS）

- [ ] **2.4 编写 TFTP 选项测试**
  - 验证 Option 66/67 正确性
  - 测试架构检测逻辑
  - Golden File 测试 DHCP 响应包

## Phase 3: ProxyDHCP 模式

- [ ] **3.1 添加 ProxyDHCP 配置**
  - 在 `Config` 中添加 `ProxyMode bool`
  - 从环境变量 `NF_DHCP_PROXY_MODE` 加载
  - 在 `DHCPServer` 中添加 `proxyMode` 字段

- [ ] **3.2 实现 ProxyDHCP 响应逻辑**
  - 修改 `handleDHCP()` 添加模式分支
  - 实现 `handleProxyDiscover()` - 仅发送引导选项
  - 在 ProxyDHCP 模式下忽略 DHCPREQUEST

- [ ] **3.3 实现 ProxyOffer 构建器**
  - 创建 `buildProxyOffer(dhcpRequest)`
  - 排除所有网络参数（IP, SubnetMask, Router, DNS）
  - 仅包含 siaddr 和 Option 66/67
  - 设置 `BroadcastAddress` 为 `255.255.255.255`

- [ ] **3.4 编写 ProxyDHCP 模式测试**
  - 测试仅响应 DISCOVER
  - 测试忽略 REQUEST
  - 测试 ProxyOffer 包含正确选项

## Phase 4: 集成和配置

- [ ] **4.1 更新配置文档**
  - 在 `config/README.md` 中添加新增环境变量说明
  - 添加 IP 池配置示例
  - 添加 ProxyDHCP 模式说明

- [ ] **4.2 更新部署脚本**
  - 在 `scripts/deploy.sh` 中添加 DHCP 配置示例
  - 添加 TFTP 服务器安装提示

- [ ] **4.3 向后兼容性验证**
  - 验证未配置 IP 池时保持原有行为
  - 验证现有部署不受影响

## Phase 5: 测试和文档

- [ ] **5.1 集成测试**
  - 完整 DHCP DORA 流程测试
  - ProxyDHCP 模式集成测试
  - 与真实 TFTP 客户端测试

- [ ] **5.2 更新 README.md**
  - 添加 IP 池管理说明
  - 添加 ProxyDHCP 使用场景
  - 添加故障排查指南

- [ ] **5.3 测试覆盖率检查**
  - 运行 `go test -cover ./internal/dhcp/...`
  - 确保覆盖率 ≥ 80%

---

## 并行化建议

- **Phase 1** 完成后，Phase 2 和 Phase 3 可以并行开发
- **Phase 4** 需要等待 Phase 1-3 完成

## 依赖关系

```
Phase 1 (IP 池管理)
    ↓
Phase 2 (TFTP 选项) ──┐
Phase 3 (ProxyDHCP)  ──┴─ 并行，依赖：Phase 1
    ↓
Phase 4 (集成配置)
    ↓
Phase 5 (测试文档)
```
