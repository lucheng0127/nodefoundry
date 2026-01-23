package dhcp

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"go.uber.org/zap"

	"github.com/lucheng0127/nodefoundry/internal/db"
	"github.com/lucheng0127/nodefoundry/internal/model"
)

// DHCPServer DHCP 服务器
type DHCPServer struct {
	addr        string
	iface       string // 绑定的网卡接口名（如 eth0），空则监听所有接口
	repo        db.NodeRepository
	logger      *zap.Logger
	server      *server4.Server
	ipManager   *IPManager
	tftpServer  string // TFTP 服务器 IP
	proxyMode   bool   // ProxyDHCP 模式
}

// NewDHCPServer 创建 DHCP 服务器
func NewDHCPServer(addr, iface string, repo db.NodeRepository, logger *zap.Logger) *DHCPServer {
	return &DHCPServer{
		addr:   addr,
		iface:  iface,
		repo:   repo,
		logger: logger,
	}
}

// SetIPManager 设置 IP 池管理器
func (s *DHCPServer) SetIPManager(ipm *IPManager) {
	s.ipManager = ipm
}

// SetTFTPServer 设置 TFTP 服务器地址
func (s *DHCPServer) SetTFTPServer(tftp string) {
	s.tftpServer = tftp
}

// SetProxyMode 设置 ProxyDHCP 模式
func (s *DHCPServer) SetProxyMode(proxy bool) {
	s.proxyMode = proxy
}

// Start 启动 DHCP 服务器
func (s *DHCPServer) Start(ctx context.Context) error {
	// 解析监听地址
	laddr, err := net.ResolveUDPAddr("udp4", s.addr)
	if err != nil {
		return fmt.Errorf("failed to resolve DHCP address: %w", err)
	}

	// 确定监听接口
	ifname := ""
	if s.iface != "" {
		ifname = s.iface
	}

	// 创建 DHCP 服务器
	s.server, err = server4.NewServer(ifname, laddr, s.handleDHCP)
	if err != nil {
		return fmt.Errorf("failed to create DHCP server: %w", err)
	}

	s.logger.Info("DHCP server starting",
		zap.String("addr", s.addr),
		zap.String("interface", s.iface),
	)

	// 在 goroutine 中启动服务器
	go func() {
		if err := s.server.Serve(); err != nil {
			s.logger.Error("DHCP server error", zap.Error(err))
		}
	}()

	// 等待 context 取消
	<-ctx.Done()

	s.logger.Info("DHCP server shutting down")
	if s.server != nil {
		s.server.Close()
	}

	return nil
}

// handleDHCP 处理 DHCP 请求
func (s *DHCPServer) handleDHCP(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) {
	if msg == nil {
		return
	}

	// ProxyDHCP 模式：只响应 DISCOVER
	if s.proxyMode {
		if msg.MessageType() == dhcpv4.MessageTypeDiscover {
			s.handleProxyDiscover(conn, peer, msg)
		}
		return
	}

	// 标准模式：处理 DISCOVER 和 REQUEST
	if msg.MessageType() != dhcpv4.MessageTypeDiscover && msg.MessageType() != dhcpv4.MessageTypeRequest {
		return
	}

	s.handleStandard(conn, peer, msg)
}

// handleStandard 标准模式处理
func (s *DHCPServer) handleStandard(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) {
	// 提取 MAC 地址
	mac := msg.ClientHWAddr.String()
	if mac == "" {
		s.logger.Warn("DHCP message with empty MAC address")
		return
	}

	// 规范化 MAC 地址
	normalizedMAC := model.NormalizeMAC(mac)

	s.logger.Debug("DHCP request received",
		zap.String("mac", normalizedMAC),
		zap.String("type", msg.MessageType().String()),
		zap.String("peer", peer.String()),
	)

	// 创建或更新节点（状态为 discovered）
	node, err := model.NewNode(normalizedMAC, model.STATE_DISCOVERED)
	if err != nil {
		s.logger.Error("failed to create node", zap.Error(err))
		return
	}

	// 如果有 IP 地址，记录下来
	if msg.ClientIPAddr != nil && !msg.ClientIPAddr.IsUnspecified() {
		node.IP = msg.ClientIPAddr.String()
	}

	// 保存到数据库
	if err := s.repo.Save(context.Background(), node); err != nil {
		s.logger.Error("failed to save node",
			zap.String("mac", normalizedMAC),
			zap.Error(err),
		)
		return
	}

	s.logger.Info("node discovered via DHCP",
		zap.String("mac", normalizedMAC),
		zap.String("ip", node.IP),
	)

	// 构建 DHCPOFFER 或 DHCPACK
	resp, err := s.buildResponse(msg)
	if err != nil {
		s.logger.Error("failed to build DHCP response", zap.Error(err))
		return
	}

	// 发送响应
	if _, err := conn.WriteTo(resp.ToBytes(), peer); err != nil {
		s.logger.Error("failed to send DHCP response",
			zap.String("mac", normalizedMAC),
			zap.Error(err),
		)
	}
}

// handleProxyDiscover ProxyDHCP 模式下的 DISCOVER 处理
func (s *DHCPServer) handleProxyDiscover(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) {
	mac := msg.ClientHWAddr.String()
	if mac == "" {
		return
	}

	normalizedMAC := model.NormalizeMAC(mac)

	s.logger.Debug("ProxyDHCP DISCOVER received",
		zap.String("mac", normalizedMAC),
		zap.String("peer", peer.String()),
	)

	// 构建 ProxyDHCPOFFER（仅包含引导选项，不含 IP）
	resp, err := s.buildProxyOffer(msg)
	if err != nil {
		s.logger.Error("failed to build ProxyDHCP offer", zap.Error(err))
		return
	}

	// 发送响应（使用广播地址）
	if _, err := conn.WriteTo(resp.ToBytes(), peer); err != nil {
		s.logger.Error("failed to send ProxyDHCP offer",
			zap.String("mac", normalizedMAC),
			zap.Error(err),
		)
	}
}

// buildResponse 构建标准 DHCP 响应
func (s *DHCPServer) buildResponse(req *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
	var respType dhcpv4.MessageType

	switch req.MessageType() {
	case dhcpv4.MessageTypeDiscover:
		respType = dhcpv4.MessageTypeOffer
	case dhcpv4.MessageTypeRequest:
		respType = dhcpv4.MessageTypeAck
	default:
		return nil, fmt.Errorf("unexpected message type: %s", req.MessageType())
	}

	resp, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		return nil, err
	}

	// 设置消息类型
	resp.UpdateOption(dhcpv4.OptMessageType(respType))

	mac := req.ClientHWAddr.String()

	// IP 分配（如果配置了 IP 池）
	if s.ipManager != nil {
		var assignedIP net.IP
		if req.MessageType() == dhcpv4.MessageTypeDiscover {
			// 分配新 IP
			assignedIP, err = s.ipManager.AllocateIP(mac, req.RequestedIPAddress())
			if err != nil {
				return nil, fmt.Errorf("failed to allocate IP: %w", err)
			}
		} else {
			// 获取已有租约
			assignedIP, err = s.ipManager.GetLease(mac)
			if err != nil {
				return nil, fmt.Errorf("failed to get lease: %w", err)
			}
		}
		resp.YourIPAddr = assignedIP

		// 设置网络参数
		resp.UpdateOption(dhcpv4.OptSubnetMask(s.ipManager.netmask))
		if s.ipManager.gateway != nil {
			resp.UpdateOption(dhcpv4.OptRouter(s.ipManager.gateway))
		}
		if len(s.ipManager.dns) > 0 {
			resp.UpdateOption(dhcpv4.OptDNS(s.ipManager.dns...))
		}
		resp.UpdateOption(dhcpv4.OptIPAddressLeaseTime(s.ipManager.leaseTime))
	} else {
		// 向后兼容：未配置 IP 池时，回显客户端请求的 IP
		if req.RequestedIPAddress() != nil {
			resp.YourIPAddr = req.RequestedIPAddress()
		} else {
			resp.YourIPAddr = req.ClientIPAddr
		}
		// 设置默认租约时间（24 小时）
		resp.UpdateOption(dhcpv4.OptIPAddressLeaseTime(24 * time.Hour))
	}

	// 设置 TFTP 引导选项
	s.setBootOptions(resp)

	return resp, nil
}

// buildProxyOffer 构建 ProxyDHCP Offer（仅包含引导选项）
func (s *DHCPServer) buildProxyOffer(req *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
	resp, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		return nil, err
	}

	// 设置消息类型为 OFFER
	resp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))

	// ProxyDHCP: 不分配 IP，仅设置引导选项
	s.setBootOptions(resp)

	// 设置广播地址
	resp.UpdateOption(dhcpv4.OptBroadcastAddress(net.IPv4bcast))

	return resp, nil
}

// setBootOptions 设置 TFTP 引导选项
func (s *DHCPServer) setBootOptions(resp *dhcpv4.DHCPv4) {
	// siaddr (Next Server) - TFTP 服务器 IP
	if s.tftpServer != "" {
		if tftpIP := net.ParseIP(s.tftpServer); tftpIP != nil {
			resp.UpdateOption(dhcpv4.OptServerIdentifier(tftpIP))
		}
	}

	// Option 66 (TFTP Server Name)
	if s.tftpServer != "" {
		resp.UpdateOption(dhcpv4.OptTFTPServerName(s.tftpServer))
	}

	// Option 67 (Bootfile Name) - 根据架构选择
	bootfile := s.getBootFile(resp)
	resp.UpdateOption(dhcpv4.OptBootFileName(bootfile))
}

// getBootFile 根据客户端架构选择引导文件
func (s *DHCPServer) getBootFile(resp *dhcpv4.DHCPv4) string {
	// 检查客户端系统架构 Option 93 (System Architecture)
	// 使用 GenericOptionCode 将整数转换为 OptionCode 类型
	if archOpt := resp.Options.Get(dhcpv4.GenericOptionCode(93)); archOpt != nil {
		if len(archOpt) >= 2 {
			arch := uint16(archOpt[0])<<8 | uint16(archOpt[1])
			// 0 = BIOS/IA32, 7 = EFI BC, 9 = EFI x86-64
			if arch == 0 {
				return "undionly.kpxe" // BIOS
			} else if arch >= 7 {
				return "ipxe.efi" // UEFI
			}
		}
	}

	// 默认返回 iPXE 链式启动文件
	return "undionly.kpxe"
}
