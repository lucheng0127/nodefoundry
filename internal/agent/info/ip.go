package info

import (
	"net"
	"strings"
)

// GetIP 获取当前主机的 IP 地址
// 返回第一个非 loopback 的 IPv4 地址
func GetIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		// 检查是否是 IP 地址
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil {
			continue
		}

		// 跳过 loopback 地址
		if ipNet.IP.IsLoopback() {
			continue
		}

		// 优先使用 IPv4
		if ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}

	return ""
}

// GetInterfaceIP 获取指定网络接口的 IP 地址
func GetInterfaceIP(ifaceName string) string {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return ""
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil {
			continue
		}

		if ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}

	return ""
}

// GetDefaultInterface 获取默认路由的网络接口名
func GetDefaultInterface() string {
	// 简单实现：返回第一个非 loopback 的接口
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// 跳过 loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// 跳过没有启用的接口
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		// 返回第一个有效的接口
		return iface.Name
	}

	return ""
}

// IsIPAddress 检查字符串是否是有效的 IP 地址
func IsIPAddress(s string) bool {
	return net.ParseIP(s) != nil
}

// IsIPv4 检查是否是 IPv4 地址
func IsIPv4(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && strings.Contains(s, ".")
}
