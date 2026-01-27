# 静态网络配置指南

## 概述

NodeFoundry 支持将 DHCP 分配的 IP 地址持久化，使安装后的系统使用静态 IP 配置。这在 NodeFoundry 作为网关（路由器）的场景下特别有用。

## 工作原理

### 标准模式（完整 DHCP）

```
┌─────────────────────────────────────────────────────────────┐
│                    NodeFoundry Server                        │
│                  (DHCP + TFTP + HTTP)                        │
└─────────────────────────────────────────────────────────────┘
                           │
                    1. DHCP DISCOVER (MAC: AABBCCDDEEFF)
                           │
                    2. DHCP OFFER (IP: 192.168.1.100)
                           │
              3. 保存网络配置到 bbolt:
                 - IP: 192.168.1.100
                 - Netmask: 255.255.255.0
                 - Gateway: 192.168.1.1
                 - DNS: 192.168.1.1
                           │
                    4. DHCP ACK
                           │
┌─────────────────────────────────────────────────────────────┐
│                         节点                                 │
│                                                              │
│  5. iPXE Boot → 下载安装脚本                                 │
│  6. 脚本传递网络参数:                                        │
│     ?ip=192.168.1.100&netmask=255.255.255.0&gateway=...     │
│  7. Preseed 生成静态网络配置                                 │
│  8. 安装 Debian，使用静态 IP                                 │
│  9. 安装后重启，继续使用相同 IP                              │
└─────────────────────────────────────────────────────────────┘
```

### ProxyDHCP 模式

在 ProxyDHCP 模式下，不分配 IP，系统安装后使用 DHCP：

```
1. DHCP DISCOVER
2. ProxyDHCP OFFER (仅引导选项，无 IP)
3. 主 DHCP 服务器分配 IP
4. 系统安装后继续使用 DHCP
```

## 配置步骤

### 1. 配置 IP 池

设置环境变量启用 IP 池管理：

```bash
# IP 池范围
export NF_DHCP_IP_POOL_START=192.168.1.100
export NF_DHCP_IP_POOL_END=192.168.1.200

# 网络参数
export NF_DHCP_NETMASK=255.255.255.0
export NF_DHCP_GATEWAY=192.168.1.1
export NF_DHCP_DNS=192.168.1.1

# 租约时间（可选，默认 86400 秒）
export NF_DHCP_LEASE_TIME=86400
```

### 2. 确保 ProxyDHCP 模式关闭

```bash
# 不要设置这个变量，或设置为 false
export NF_DHCP_PROXY_MODE=false
```

### 3. 重启 NodeFoundry Server

```bash
sudo systemctl restart nodefoundry
```

### 4. 验证配置

检查服务器日志：

```bash
journalctl -u nodefoundry -f | grep -i dhcp
```

应该看到类似的日志：

```
DHCP request received mac=aabbccddeeff type=DISCOVER
Allocated new IP mac=aabbccddeeff ip=192.168.1.100
network config saved mac=aabbccddeeff ip=192.168.1.100 netmask=255.255.255.0 gateway=192.168.1.1 dns=192.168.1.1
DHCP response sent mac=aabbccddeeff ip=192.168.1.100 type=OFFER
```

## IP 分配策略

### 固定 IP 分配

每个 MAC 地址在 IP 池中获得固定 IP：

1. **首次分配**: 从 IP 池起始地址开始分配
2. **续租**: 返回相同的 IP（基于 bbolt 持久化）
3. **IP 池耗尽**: 拒绝新节点，记录错误日志

### IP 持久化

网络配置保存在两个位置：

1. **节点记录** (bbolt `nodes` bucket):
   ```json
   {
     "mac": "aabbccddeeff",
     "ip": "192.168.1.100",
     "netmask": "255.255.255.0",
     "gateway": "192.168.1.1",
     "dns": "192.168.1.1"
   }
   ```

2. **IP 租约** (内存 + bbolt `ip_allocations` bucket):
   - MAC → IP 映射
   - 服务重启后从 bbolt 恢复

## Preseed 集成

### iPXE 脚本生成

iPXE 脚本自动包含网络参数：

```ipxe
#!ipxe
set node_url http://192.168.1.10:8080
set mac aabbccddeeff

kernel http://mirrors.ustc.edu.cn/debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/linux \
  auto=true priority=critical \
  url=${node_url}/preseed/${mac}?ip=192.168.1.100&netmask=255.255.255.0&gateway=192.168.1.1&dns=192.168.1.1

initrd http://mirrors.ustc.edu.cn/debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz
boot
```

### Preseed 静态配置

Preseed 生成器根据查询参数生成静态网络配置：

```bash
# 静态配置
d-i netcfg/disable_autoconfig boolean true
d-i netcfg/disable_dhcp boolean true
d-i netcfg/get_ipaddress string 192.168.1.100
d-i netcfg/get_netmask string 255.255.255.0
d-i netcfg/get_gateway string 192.168.1.1
d-i netcfg/get_nameservers string 192.168.1.1
d-i netcfg/confirm_static boolean true
```

## API 操作

### 查询节点网络配置

```bash
curl http://localhost:8080/api/v1/nodes/aabbccddeeff
```

响应：

```json
{
  "mac": "aabbccddeeff",
  "ip": "192.168.1.100",
  "netmask": "255.255.255.0",
  "gateway": "192.168.1.1",
  "dns": "192.168.1.1",
  "status": "installed"
}
```

### 手动设置网络配置

```bash
curl -X PUT http://localhost:8080/api/v1/nodes/aabbccddeeff \
  -H "Content-Type: application/json" \
  -d '{
    "ip": "192.168.1.150",
    "netmask": "255.255.255.0",
    "gateway": "192.168.1.1",
    "dns": "192.168.1.1"
  }'
```

## 验证静态配置

### 在已安装节点上验证

登录已安装的节点，检查网络配置：

#### Debian 12 (NetworkManager)

```bash
# 查看连接
nmcli connection show

# 查看详细信息
nmcli connection show "Wired connection 1"
```

#### Debian 12 (传统方式)

```bash
# 查看 /etc/network/interfaces
cat /etc/network/interfaces
```

应该看到：

```bash
auto eth0
iface eth0 inet static
    address 192.168.1.100
    netmask 255.255.255.0
    gateway 192.168.1.1
    dns-nameservers 192.168.1.1
```

### 验证 IP 地址

```bash
ip addr show eth0
```

### 验证路由

```bash
ip route show
```

应该看到：

```
default via 192.168.1.1 dev eth0
192.168.1.0/24 dev eth0 proto kernel scope link src 192.168.1.100
```

### 验证 DNS

```bash
cat /etc/resolv.conf
```

## 故障排查

### IP 池耗尽

**症状**: 日志显示 "IP pool exhausted"

**解决**:
1. 扩大 IP 池范围
2. 回收不用的节点 IP
3. 检查是否有重复的 MAC 地址

### IP 冲突

**症状**: 节点无法访问网络，ARP 冲突

**解决**:
1. 检查网络中是否有手动分配的相同 IP
2. 使用 `arp-scan` 或 `nmap` 扫描网络
3. 修改节点的 IP 配置

### 网络配置未生效

**症状**: 安装后使用 DHCP 而非静态 IP

**检查**:
1. 查看 preseed URL 是否包含网络参数
2. 检查安装日志：`/var/log/syslog`
3. 验证 `/etc/network/interfaces` 配置

### ProxyDHCP 模式误配置

**症状**: 节点获得 IP，但安装后配置为 DHCP

**检查**:
1. 确认 `NF_DHCP_PROXY_MODE=false`
2. 查看服务器日志中是否为 "ProxyDHCP mode"
3. 验证节点记录中是否有 IP 字段

## 高级配置

### 自定义子网

```bash
# /24 网络 (254 个可用 IP)
export NF_DHCP_NETMASK=255.255.255.0
export NF_DHCP_IP_POOL_START=192.168.1.100
export NF_DHCP_IP_POOL_END=192.168.1.254

# /16 网络 (65534 个可用 IP)
export NF_DHCP_NETMASK=255.255.0.0
export NF_DHCP_IP_POOL_START=192.168.10.1
export NF_DHCP_IP_POOL_END=192.168.20.254
```

### 多个 DNS 服务器

```bash
export NF_DHCP_DNS=8.8.8.8,8.8.4.4,1.1.1.1
```

### VLAN 支持

如果使用 VLAN，需要确保 DHCP 服务器监听正确的 VLAN 接口：

```bash
export NF_DHCP_INTERFACE=eth0.100
```

## 迁移指南

### 从 DHCP 迁移到静态 IP

对于已安装并使用 DHCP 的节点：

1. **通过 API 更新节点配置**
2. **手动登录节点修改网络配置**
3. **重启网络服务**

```bash
# 在节点上
sudo nano /etc/network/interfaces
# 修改为静态配置

sudo systemctl restart networking
```

### 从静态 IP 迁移到 DHCP

1. **清除节点记录中的 IP 字段**
2. **设置 ProxyDHCP 模式**（如果需要）
3. **重新安装节点**

## 最佳实践

1. **IP 池规划**: 预留足够的 IP 地址，考虑未来扩展
2. **文档记录**: 记录 IP 分配策略和网络拓扑
3. **监控**: 监控 IP 池使用情况，设置告警
4. **备份**: 定期备份 bbolt 数据库
5. **测试**: 在生产环境前测试网络配置

## 注意事项

1. **网络环境**: 确保网络中没有其他 DHCP 服务器
2. **防火墙**: 开放 DHCP 端口（UDP 67/68）
3. **网关配置**: 确保网关地址正确且可达
4. **DNS 配置**: 验证 DNS 服务器可用
5. **子网规划**: IP 池应在同一子网内

## 相关文档

- [DHCP 配置模式](../README.md#dhcp-配置模式)
- [Agent 部署](AGENT.md)
- [API 文档](../README.md#api-文档)
