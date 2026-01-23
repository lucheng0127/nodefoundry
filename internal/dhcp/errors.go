package dhcp

import "errors"

var (
	// ErrInvalidIPStart 无效的 IP 池起始地址
	ErrInvalidIPStart = errors.New("invalid IP pool start address")
	// ErrInvalidIPEnd 无效的 IP 池结束地址
	ErrInvalidIPEnd = errors.New("invalid IP pool end address")
	// ErrInvalidNetmask 无效的子网掩码
	ErrInvalidNetmask = errors.New("invalid netmask")
	// ErrInvalidGateway 无效的网关地址
	ErrInvalidGateway = errors.New("invalid gateway address")
	// ErrInvalidDNS 无效的 DNS 地址
	ErrInvalidDNS = errors.New("invalid DNS address")
	// ErrInvalidIP 无效的 IP 地址
	ErrInvalidIP = errors.New("invalid IP address")
	// ErrIPPoolExhausted IP 池已耗尽
	ErrIPPoolExhausted = errors.New("IP pool exhausted")
	// ErrIPNotAllocated IP 未分配
	ErrIPNotAllocated = errors.New("IP not allocated")
	// ErrLeaseNotFound 租约不存在
	ErrLeaseNotFound = errors.New("lease not found")
	// ErrLeaseExpired 租约已过期
	ErrLeaseExpired = errors.New("lease expired")
	// ErrInterfaceNotFound 网卡接口未找到
	ErrInterfaceNotFound = errors.New("interface not found")
)
