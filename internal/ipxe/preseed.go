package ipxe

import (
	"context"
	"fmt"

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
	mac = model.NormalizeMAC(mac)

	node, err := g.repo.FindByMAC(ctx, mac)
	if err != nil {
		return "", err
	}

	hostname := g.getHostname(node)

	preseed := fmt.Sprintf(`d-i debian-installer/locale string en_US
d-i keyboard-configuration/xkb-keymap select us
d-i netcfg/choose_interface select auto
d-i netcfg/get_hostname string %s
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
d-i preseed/late_command string in-target /bin/sh -c "apt-get install -y curl mosquitto-clients; curl -fsSL http://%s/agent/nodefoundry-agent -o /usr/local/bin/nodefoundry-agent; chmod +x /usr/local/bin/nodefoundry-agent; curl -fsSL http://%s/agent/nodefoundry-agent.service -o /etc/systemd/system/nodefoundry-agent.service; systemctl enable nodefoundry-agent.service"
`, hostname, g.mirrorURL, g.serverAddr, g.serverAddr)

	return preseed, nil
}

// getHostname 获取节点主机名
func (g *PreseedGenerator) getHostname(node *model.Node) string {
	if node.Hostname != "" {
		return node.Hostname
	}
	return "node-" + node.MAC
}
