package agent

import (
	"fmt"
	"net"
	"strings"
)

// GetMACAddress 获取 MAC 地址
// 优先使用环境变量，回退到自动检测第一个有效的网络接口
func GetMACAddress(envMAC string) (string, error) {
	// 如果环境变量中有 MAC，直接使用（先验证格式）
	if envMAC != "" {
		mac := normalizeMAC(envMAC)
		if isValidMAC(mac) {
			return mac, nil
		}
		return "", fmt.Errorf("invalid MAC address in environment: %s", envMAC)
	}

	// 自动检测：遍历网络接口，选择第一个有效的接口
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to list network interfaces: %w", err)
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

		// 跳过没有 MAC 地址的接口
		if len(iface.HardwareAddr) == 0 {
			continue
		}

		// 返回第一个有效接口的 MAC
		mac := normalizeMAC(iface.HardwareAddr.String())
		if isValidMAC(mac) {
			return mac, nil
		}
	}

	return "", fmt.Errorf("no valid network interface found")
}

// normalizeMAC 标准化 MAC 地址为小写、无分隔符格式
func normalizeMAC(mac string) string {
	// 移除所有非字母数字字符
	result := make([]byte, 0, 12)
	for _, c := range mac {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			result = append(result, byte(c))
		}
	}
	// 转换为小写
	for i := range result {
		if result[i] >= 'A' && result[i] <= 'F' {
			result[i] += 32
		}
	}
	return string(result)
}

// isValidMAC 验证 MAC 地址格式
func isValidMAC(mac string) bool {
	// 标准化后的 MAC 应该是 12 位十六进制字符
	if len(mac) != 12 {
		return false
	}

	for _, c := range mac {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}

	return true
}

// FormatMAC 格式化 MAC 地址为标准格式（XX:XX:XX:XX:XX:XX）
func FormatMAC(mac string) string {
	if len(mac) != 12 {
		return mac
	}

	var result strings.Builder
	for i := 0; i < 12; i += 2 {
		if i > 0 {
			result.WriteByte(':')
		}
		result.WriteString(strings.ToUpper(mac[i : i+2]))
	}

	return result.String()
}
