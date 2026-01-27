package ipxe

import (
	"context"
	"fmt"
	"net/url"

	"go.uber.org/zap"

	"github.com/lucheng0127/nodefoundry/internal/db"
	"github.com/lucheng0127/nodefoundry/internal/model"
)

// PreseedGenerator preseed 配置生成器
type PreseedGenerator struct {
	serverAddr string
	mirrorURL  string
	repo       db.NodeRepository
	logger     *zap.Logger
}

// NewPreseedGenerator 创建 preseed 生成器
func NewPreseedGenerator(serverAddr string, mirrorURL string, repo db.NodeRepository, logger *zap.Logger) *PreseedGenerator {
	return &PreseedGenerator{
		serverAddr: serverAddr,
		mirrorURL:  mirrorURL,
		repo:       repo,
		logger:     logger,
	}
}

// Generate 生成 preseed 配置
func (g *PreseedGenerator) Generate(ctx context.Context, mac string) (string, error) {
	return g.GenerateWithQuery(ctx, mac, url.Values{})
}

// GenerateWithQuery 生成 preseed 配置（支持查询参数）
func (g *PreseedGenerator) GenerateWithQuery(ctx context.Context, mac string, query url.Values) (string, error) {
	mac = model.NormalizeMAC(mac)

	node, err := g.repo.FindByMAC(ctx, mac)
	if err != nil {
		return "", err
	}

	hostname := g.getHostname(node)

	// 获取网络配置参数（优先使用查询参数，回退到节点记录）
	ip := query.Get("ip")
	netmask := query.Get("netmask")
	gateway := query.Get("gateway")
	dns := query.Get("dns")

	// 如果查询参数为空，使用节点记录中的配置
	if ip == "" && node.IP != "" {
		ip = node.IP
		netmask = node.Netmask
		gateway = node.Gateway
		dns = node.DNS
	}

	// 如果有静态网络配置，生成静态配置部分
	netcfgSection := ""
	if ip != "" {
		// 静态网络配置
		netcfgSection = fmt.Sprintf(`
# Static network configuration
d-i netcfg/disable_autoconfig boolean true
d-i netcfg/disable_dhcp boolean true
d-i netcfg/get_ipaddress string %s`, ip)

		if netmask != "" {
			netcfgSection += fmt.Sprintf(`
d-i netcfg/get_netmask string %s`, netmask)
		}

		if gateway != "" {
			netcfgSection += fmt.Sprintf(`
d-i netcfg/get_gateway string %s`, gateway)
		}

		if dns != "" {
			netcfgSection += fmt.Sprintf(`
d-i netcfg/get_nameservers string %s`, dns)
		}

		netcfgSection += `
d-i netcfg/confirm_static boolean true`
	} else {
		// DHCP 配置（默认）
		netcfgSection = `
# DHCP network configuration (default)
d-i netcfg/disable_dhcp boolean false`
	}

	// 生成 late_command（Agent 安装 + MAC 地址注入）
	lateCommand := g.generateLateCommand()

	preseed := fmt.Sprintf(`d-i debian-installer/locale string en_US
d-i keyboard-configuration/xkb-keymap select us
d-i netcfg/choose_interface select auto
d-i netcfg/get_hostname string %s
%s
d-i mirror/country string manual
d-i mirror/http/hostname string %s
d-i mirror/http/directory string /debian
d-i mirror/http/proxy string
d-i time/zone string Asia/Shanghai
d-i clock-setup/utc-auto boolean true
d-i clock-setup/utc boolean true
d-i partman-auto/method string regular
d-i partman-lvm/device_remove_lvm boolean true
d-i partman-md/device_remove_md boolean true
d-i partman-lvm/confirm boolean true
d-i partman-partitioning/confirm_write_new_label boolean true
d-i partman/choose_partition select finish
d-i partman/confirm boolean true
d-i partman/confirm_nooverwrite boolean true
d-i pkgsel/upgrade select none
d-i grub-installer/only_debian boolean true
d-i grub-installer/with_other_os boolean true
d-i grub-installer/bootdev string default
d-i finish-install/reboot_in_progress note

# Install NodeFoundry agent
%s
`, hostname, netcfgSection, g.mirrorURL, lateCommand)

	return preseed, nil
}

// getHostname 获取节点主机名
func (g *PreseedGenerator) getHostname(node *model.Node) string {
	if node.Hostname != "" {
		return node.Hostname
	}
	return "node-" + node.MAC
}

// generateLateCommand 生成 late_command（Agent 安装 + MAC 地址注入）
func (g *PreseedGenerator) generateLateCommand() string {
	return fmt.Sprintf(`d-i preseed/late_command string \
  DHCP_iface=$(ip route | grep default | awk '{print $$5}') && \
  DHCP_MAC=$$(cat /sys/class/net/$${DHCP_iface}/address | tr -d ':') && \
  echo "Detected DHCP MAC: $${DHCP_MAC}" > /target/var/log/nodefoundry-agent-install.log && \
  in-target wget http://%s/agent/nodefoundry-agent -O /usr/local/bin/nodefoundry-agent && \
  in-target chmod +x /usr/local/bin/nodefoundry-agent && \
  in-target wget http://%s/agent/nodefoundry-agent.service -O /etc/systemd/system/nodefoundry-agent.service && \
  in-target sh -c 'echo "NF_MAC=$${DHCP_MAC}" > /etc/default/nodefoundry-agent' && \
  in-target sh -c 'echo "NF_MQTT_BROKER=%s:1883" >> /etc/default/nodefoundry-agent' && \
  in-target sh -c 'echo "NF_LOG_LEVEL=info" >> /etc/default/nodefoundry-agent' && \
  in-target sh -c 'echo "NF_HEARTBEAT_INTERVAL=30" >> /etc/default/nodefoundry-agent' && \
  in-target systemctl enable nodefoundry-agent.service`, g.serverAddr, g.serverAddr, g.getServerIP())
}

// getServerIP 从 serverAddr 中提取 IP 地址
func (g *PreseedGenerator) getServerIP() string {
	// 简单提取：从 serverAddr 中提取 IP（去掉端口）
	// 例如: "192.168.1.100:8080" -> "192.168.1.100"
	for i, c := range g.serverAddr {
		if c == ':' {
			return g.serverAddr[:i]
		}
	}
	return g.serverAddr
}
