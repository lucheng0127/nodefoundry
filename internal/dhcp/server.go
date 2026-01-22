package dhcp

import (
	"context"
	"fmt"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"go.uber.org/zap"

	"github.com/lucheng0127/nodefoundry/internal/db"
	"github.com/lucheng0127/nodefoundry/internal/model"
)

// DHCPServer DHCP 服务器
type DHCPServer struct {
	addr   string
	repo   db.NodeRepository
	logger *zap.Logger
	server *server4.Server
}

// NewDHCPServer 创建 DHCP 服务器
func NewDHCPServer(addr string, repo db.NodeRepository, logger *zap.Logger) *DHCPServer {
	return &DHCPServer{
		addr:   addr,
		repo:   repo,
		logger: logger,
	}
}

// Start 启动 DHCP 服务器
func (s *DHCPServer) Start(ctx context.Context) error {
	// 解析监听地址
	laddr, err := net.ResolveUDPAddr("udp4", s.addr)
	if err != nil {
		return fmt.Errorf("failed to resolve DHCP address: %w", err)
	}

	// 创建 DHCP 服务器
	// 空字符串 ifname 表示监听所有接口
	s.server, err = server4.NewServer("", laddr, s.handleDHCP)
	if err != nil {
		return fmt.Errorf("failed to create DHCP server: %w", err)
	}

	s.logger.Info("DHCP server starting", zap.String("addr", s.addr))

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

	// 只处理 DHCPDISCOVER 和 DHCPREQUEST
	if msg.MessageType() != dhcpv4.MessageTypeDiscover && msg.MessageType() != dhcpv4.MessageTypeRequest {
		return
	}

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

// buildResponse 构建 DHCP 响应
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

	// 构建响应
	// 使用 WithOption 方法设置响应类型
	resp, err := dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		return nil, err
	}

	// 设置消息类型
	resp.UpdateOption(dhcpv4.OptMessageType(respType))

	// 如果请求中有特定 IP，分配该 IP；否则分配新 IP
	if req.RequestedIPAddress() != nil {
		resp.YourIPAddr = req.RequestedIPAddress()
	} else {
		// 使用客户端 IP 作为分配的 IP
		resp.YourIPAddr = req.ClientIPAddr
	}

	// 设置租约时间（24 小时）
	resp.UpdateOption(dhcpv4.OptIPAddressLeaseTime(24 * 60 * 60))

	// 添加 TFTP 服务器选项（用于 iPXE）
	if tftpAddr := s.getTFTPServerAddress(); tftpAddr != "" {
		resp.UpdateOption(dhcpv4.OptTFTPServerName(tftpAddr))
	}

	// 添加引导文件名选项
	resp.BootFileName = "undionly.kpxe"

	return resp, nil
}

// getTFTPServerAddress 获取 TFTP 服务器地址
// 从 DHCP 监听地址推断
func (s *DHCPServer) getTFTPServerAddress() string {
	// 如果 DHCP 监听在 :67，使用主机的 IP
	// 这里简化处理，返回空字符串，让 DHCP 客户端使用默认值
	// 实际部署时，应该返回 TFTP 服务器的 IP 地址
	return ""
}
