package ipxe

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/lucheng0127/nodefoundry/internal/db"
	"github.com/lucheng0127/nodefoundry/internal/model"
)

// Generator iPXE 脚本生成器
type Generator struct {
	serverAddr string
	mirrorURL  string
	repo       db.NodeRepository
	logger     *zap.Logger
}

// NewGenerator 创建 iPXE 脚本生成器
func NewGenerator(serverAddr string, mirrorURL string, repo db.NodeRepository, logger *zap.Logger) *Generator {
	return &Generator{
		serverAddr: serverAddr,
		mirrorURL:  mirrorURL,
		repo:       repo,
		logger:     logger,
	}
}

// GenerateByStatus 根据节点状态生成 iPXE 脚本
func (g *Generator) GenerateByStatus(ctx context.Context, mac string) (string, error) {
	mac = model.NormalizeMAC(mac)

	node, err := g.repo.FindByMAC(ctx, mac)
	if err != nil {
		return "", err
	}

	switch node.Status {
	case model.STATE_DISCOVERED:
		return g.generateWaitLoopScript(mac), nil
	case model.STATE_INSTALLING:
		return g.generateInstallScript(mac), nil
	case model.STATE_INSTALLED:
		return g.generateLocalBootScript(), nil
	default:
		return "", fmt.Errorf("unknown node status: %s", node.Status)
	}
}

// generateWaitLoopScript 生成等待循环脚本
func (g *Generator) generateWaitLoopScript(mac string) string {
	return fmt.Sprintf(`#!ipxe
set node_url http://%s
set mac %s

:loop
echo Node in discovered state, waiting for installation trigger...
sleep 90
chain ${node_url}/boot/${mac}/boot.ipxe || goto loop
`, g.serverAddr, mac)
}

// generateInstallScript 生成安装脚本
func (g *Generator) generateInstallScript(mac string) string {
	return fmt.Sprintf(`#!ipxe
set node_url http://%s
set mac %s
set arch ${buildarch}

kernel https://%s/debian/dists/bookworm/main/installer-${arch}/current/images/netboot/debian-installer/${arch}/linux
initrd https://%s/debian/dists/bookworm/main/installer-${arch}/current/images/netboot/debian-installer/${arch}/initrd.gz
imgargs linux auto=true priority=critical url=${node_url}/preseed/${mac}/preseed.cfg
boot
`, g.serverAddr, mac, g.mirrorURL, g.mirrorURL)
}

// generateLocalBootScript 生成本地启动脚本
func (g *Generator) generateLocalBootScript() string {
	return `#!ipxe
echo Booting from local disk...
exit
`
}
