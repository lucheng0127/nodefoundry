package dhcp

import (
	"encoding/binary"
	"net"
	"sync"
	"time"
)

// IPManager IP 地址池管理器
type IPManager struct {
	start     net.IP
	end       net.IP
	netmask   net.IPMask
	gateway   net.IP
	dns       []net.IP
	leaseTime time.Duration

	// 租约管理：mac → *Lease
	leases map[string]*Lease

	// 反向索引：ip → mac
	allocated map[string]string

	mu sync.RWMutex
}

// Lease IP 租约
type Lease struct {
	MAC       string
	IP        net.IP
	ExpiresAt time.Time
}

// NewIPManager 创建 IP 池管理器
func NewIPManager(start, end, netmask, gateway string, dns []string, leaseTimeSec int) (*IPManager, error) {
	startIP := net.ParseIP(start)
	if startIP == nil {
		return nil, ErrInvalidIPStart
	}
	endIP := net.ParseIP(end)
	if endIP == nil {
		return nil, ErrInvalidIPEnd
	}

	maskIP := net.ParseIP(netmask)
	if maskIP == nil {
		return nil, ErrInvalidNetmask
	}
	ipMask := net.IPMask(maskIP.To4())

	gw := net.ParseIP(gateway)
	if gateway != "" && gw == nil {
		return nil, ErrInvalidGateway
	}

	dnsIPs := make([]net.IP, 0, len(dns))
	for _, d := range dns {
		dnsIP := net.ParseIP(d)
		if dnsIP == nil {
			return nil, ErrInvalidDNS
		}
		if ipv4 := dnsIP.To4(); ipv4 != nil {
			dnsIPs = append(dnsIPs, ipv4)
		}
	}

	return &IPManager{
		start:     startIP.To4(),
		end:       endIP.To4(),
		netmask:   ipMask,
		gateway:   gw.To4(),
		dns:       dnsIPs,
		leaseTime: time.Duration(leaseTimeSec) * time.Second,
		leases:    make(map[string]*Lease),
		allocated: make(map[string]string),
	}, nil
}

// AllocateIP 为 MAC 地址分配 IP
func (m *IPManager) AllocateIP(mac string, requestedIP net.IP) (net.IP, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 规范化 MAC 地址
	normalizedMAC := normalizeMAC(mac)

	// 检查是否已有租约
	if existing, ok := m.leases[normalizedMAC]; ok {
		// 更新租约时间
		existing.ExpiresAt = time.Now().Add(m.leaseTime)
		return existing.IP, nil
	}

	// 如果请求了特定 IP，检查是否可用
	if requestedIP != nil && !requestedIP.IsUnspecified() {
		reqIP := requestedIP.To4()
		if m.isIPInPool(reqIP) {
			if !m.isIPAllocated(reqIP) {
				return m.allocateIP(normalizedMAC, reqIP), nil
			}
		}
	}

	// 分配池中下一个可用 IP
	for ip := m.ipToInt(m.start); ip <= m.ipToInt(m.end); ip++ {
		candidate := m.intToIP(ip)
		if !m.isIPAllocated(candidate) {
			return m.allocateIP(normalizedMAC, candidate), nil
		}
	}

	return nil, ErrIPPoolExhausted
}

// ReleaseIP 释放 IP
func (m *IPManager) ReleaseIP(ip net.IP) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ip == nil {
		return ErrInvalidIP
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return ErrInvalidIP
	}

	ipStr := ipv4.String()
	mac, ok := m.allocated[ipStr]
	if !ok {
		return ErrIPNotAllocated
	}

	// 删除租约
	delete(m.leases, mac)
	delete(m.allocated, ipStr)

	return nil
}

// RenewLease 续租
func (m *IPManager) RenewLease(mac string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	normalizedMAC := normalizeMAC(mac)
	lease, ok := m.leases[normalizedMAC]
	if !ok {
		return ErrLeaseNotFound
	}

	lease.ExpiresAt = time.Now().Add(m.leaseTime)
	return nil
}

// GetLease 获取 MAC 的租约 IP
func (m *IPManager) GetLease(mac string) (net.IP, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedMAC := normalizeMAC(mac)
	lease, ok := m.leases[normalizedMAC]
	if !ok {
		return nil, ErrLeaseNotFound
	}

	// 检查租约是否过期
	if time.Now().After(lease.ExpiresAt) {
		return nil, ErrLeaseExpired
	}

	return lease.IP, nil
}

// allocateIP 内部方法：分配 IP 并创建租约
func (m *IPManager) allocateIP(mac string, ip net.IP) net.IP {
	m.leases[mac] = &Lease{
		MAC:       mac,
		IP:        ip,
		ExpiresAt: time.Now().Add(m.leaseTime),
	}
	m.allocated[ip.String()] = mac
	return ip
}

// isIPInPool 检查 IP 是否在池范围内
func (m *IPManager) isIPInPool(ip net.IP) bool {
	if ip == nil {
		return false
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return false
	}

	ipInt := m.ipToInt(ipv4)
	startInt := m.ipToInt(m.start)
	endInt := m.ipToInt(m.end)

	return ipInt >= startInt && ipInt <= endInt
}

// isIPAllocated 检查 IP 是否已分配
func (m *IPManager) isIPAllocated(ip net.IP) bool {
	if ip == nil {
		return false
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return false
	}
	_, ok := m.allocated[ipv4.String()]
	return ok
}

// ipToInt 将 IP 转换为整数（用于比较和遍历）
func (m *IPManager) ipToInt(ip net.IP) uint32 {
	return binary.BigEndian.Uint32(ip)
}

// intToIP 将整数转换为 IP
func (m *IPManager) intToIP(n uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}

// normalizeMAC 规范化 MAC 地址格式
func normalizeMAC(mac string) string {
	hwAddr, err := net.ParseMAC(mac)
	if err != nil {
		return mac
	}
	return hwAddr.String()
}
