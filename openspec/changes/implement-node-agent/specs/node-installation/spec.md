## MODIFIED Requirements

### Requirement: 动态生成 preseed 配置文件

The system SHALL dynamically generate preseed configuration files for each node.

The preseed configuration SHALL:
1. Use USTC mirror as package source
2. Complete installation automatically without user interaction
3. **Configure static network if node has DHCP-allocated IP**
4. Install and configure NodeFoundry agent via late_command
5. Configure agent as systemd service

#### Scenario: 请求节点的 preseed 配置文件（带静态网络）

**Given** 节点（MAC: AABBCCDDEEFF）已存在于数据库中
**And** 节点记录包含网络配置：
  - IP: `192.168.1.100`
  - Netmask: `255.255.255.0`
  - Gateway: `192.168.1.1`
  - DNS: `192.168.1.1`
**When** 请求 `/preseed/aabbccddeeff/preseed.cfg`
**Then** 系统应当返回包含静态网络配置的 preseed：
```bash
d-i debian-installer/locale string en_US
d-i keyboard-configuration/xkb-keymap select us

# 静态网络配置
d-i netcfg/choose_interface select auto
d-i netcfg/disable_autoconfig boolean true
d-i netcfg/get_ipaddress string 192.168.1.100
d-i netcfg/get_netmask string 255.255.255.0
d-i netcfg/get_gateway string 192.168.1.1
d-i netcfg/get_nameservers string 192.168.1.1
d-i netcfg/confirm_static boolean true

d-i netcfg/get_hostname string node-aabbccddeeff
d-i netcfg/get_domain string

# 镜像配置
d-i mirror/country string manual
d-i mirror/http/hostname string mirrors.ustc.edu.cn
d-i mirror/http/directory string /debian
d-i mirror/http/proxy string

# 时区和时钟
d-i time/zone string Asia/Shanghai
d-i clock-setup/utc-auto boolean true
d-i clock-setup/utc boolean true

# 分区配置
d-i partman-auto/method string regular
d-i partman-lvm/device_remove_lvm boolean true
d-i partman-md/device_remove_md boolean true
d-i partman-lvm/confirm boolean true
d-i partman-partitioning/confirm_write_new_label boolean true
d-i partman/choose_partition select finish
d-i partman/confirm boolean true
d-i partman/confirm_nooverwrite boolean true

# 软件包选择
d-i pkgsel/upgrade select none
tasksel tasksel/first multiselect standard, ssh-server
d-i pkgsel/include string curl mosquitto-clients

# GRUB 引导器
d-i grub-installer/only_debian boolean true
d-i grub-installer/with_other_os boolean true
d-i grub-installer/bootdev string default

# 安装完成
d-i finish-install/reboot_in_progress note
```

#### Scenario: 请求节点的 preseed 配置文件（无静态网络，使用 DHCP）

**Given** 节点（MAC: AABBCCDDEEFF）已存在于数据库中
**And** 节点记录**不包含**网络配置（IP 为空）
**When** 请求 `/preseed/aabbccddeeff/preseed.cfg`
**Then** 系统应当返回使用 DHCP 的 preseed：
```bash
d-i debian-installer/locale string en_US
d-i keyboard-configuration/xkb-keymap select us

# DHCP 网络配置
d-i netcfg/choose_interface select auto
d-i netcfg/get_hostname string node-aabbccddeeff
d-i netcfg/get_domain string

# ... 其余配置与上面相同 ...
```

#### Scenario: preseed 中包含 Agent 安装（静态网络场景）

**Given** 节点（MAC: AABBCCDDEEFF）有静态网络配置
  - IP: `192.168.1.100`
  - Gateway: `192.168.1.1`
  - DHCP 网卡: eth0
**When** 生成 preseed 的 late_command
**Then** late_command 应当：
```bash
d-i preseed/late_command string \
  # 获取 DHCP 网卡的 MAC 地址（安装阶段网络接口）
  DHCP_iface=$(ip route | grep default | awk '{print $5}') && \
  DHCP_MAC=$(cat /sys/class/net/${DHCP_iface}/address | tr -d ':') && \
  echo "Detected DHCP MAC: ${DHCP_MAC}" > /target/var/log/nodefoundry-agent-install.log && \
  \
  # 下载并安装 Agent
  in-target wget http://${server}:8080/agent/nodefoundry-agent -O /usr/local/bin/nodefoundry-agent && \
  in-target chmod +x /usr/local/bin/nodefoundry-agent && \
  in-target wget http://${server}:8080/agent/nodefoundry-agent.service -O /etc/systemd/system/nodefoundry-agent.service && \
  \
  # 生成环境变量文件，包含 DHCP MAC
  in-target sh -c 'echo "NF_MAC='${DHCP_MAC}'" > /etc/default/nodefoundry-agent' && \
  in-target sh -c 'echo "NF_MQTT_BROKER='${server}':1883" >> /etc/default/nodefoundry-agent' && \
  in-target sh -c 'echo "NF_LOG_LEVEL=info" >> /etc/default/nodefoundry-agent' && \
  in-target sh -c 'echo "NF_HEARTBEAT_INTERVAL=30" >> /etc/default/nodefoundry-agent' && \
  \
  # 启用服务
  in-target systemctl enable nodefoundry-agent.service
```

### Requirement: 生成 NetworkManager 静态网络配置

The system SHALL support generating NetworkManager configuration for static IP in preseed (optional alternative to netcfg).

#### Scenario: 使用 NetworkManager 配置静态网络

**Given** 节点（MAC: AABBCCDDEEFF）有静态网络配置
**When** 生成 preseed 使用 NetworkManager
**Then** late_command 应当创建 NetworkManager 连接配置：
```bash
# 在 late_command 中添加
in-target nmcli connection add type ethernet con-name static \
  ifname $(ip route | grep default | awk '{print $5}') \
  ip4 192.168.1.100/24 \
  gw4 192.168.1.1 && \
in-target nmcli connection modify static ipv4.dns "192.168.1.1" && \
in-target nmcli connection up static
```

### Requirement: 节点安装后网络配置验证

The agent SHALL verify network configuration after installation and report via MQTT.

#### Scenario: Agent 上报静态网络配置

**Given** 节点使用静态网络配置安装完成
**And** 静态配置为：IP=192.168.1.100, Gateway=192.168.1.1
**When** Agent 首次启动并上报状态
**Then** MQTT 消息 payload 应当包含：
```json
{
  "status": "installed",
  "ip": "192.168.1.100",
  "hostname": "node-aabbccddeeff",
  "uptime": 0,
  "timestamp": "2024-01-23T10:30:00Z",
  "network_config": {
    "type": "static",
    "ip": "192.168.1.100",
    "netmask": "255.255.255.0",
    "gateway": "192.168.1.1",
    "dns": "192.168.1.1"
  }
}
```
