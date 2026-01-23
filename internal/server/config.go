package server

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 服务配置
type Config struct {
	// HTTP 服务地址
	HTTPAddr string
	// DHCP 服务地址
	DHCPAddr string
	// DHCP 绑定的网卡接口
	DHCPInterface string
	// DHCP TFTP 服务器地址
	DHCPTFTPServer string
	// DHCP ProxyDHCP 模式
	DHCPProxyMode bool
	// MQTT Broker 地址
	MQTTBroker string
	// Debian 镜像源
	MirrorURL string
	// 数据库路径
	DBPath string
	// 日志级别
	LogLevel string
	// iPXE 脚本中的服务器地址
	ServerAddr string
	// IP 池配置
	DHCPIPPoolStart string
	DHCPIPPoolEnd   string
	DHCPNetmask     string
	DHCPGateway     string
	DHCPDNS         []string
	DHCPLeaseTime   int
}

// LoadConfig 从环境变量加载配置
func LoadConfig() *Config {
	httpAddr := getEnv("NF_HTTP_ADDR", ":8080")
	mirrorURL := getEnv("NF_MIRROR_URL", "mirrors.ustc.edu.cn")
	serverAddr := getEnv("NF_SERVER_ADDR", "")

	// 如果未设置 ServerAddr，从 HTTPAddr 推断
	if serverAddr == "" {
		if httpAddr[0] == ':' {
			serverAddr = "localhost" + httpAddr
		} else {
			serverAddr = httpAddr
		}
	}

	// 解析 DHCP DNS 列表
	dhcpDNS := parseDNSList(getEnv("NF_DHCP_DNS", "8.8.8.8,8.8.4.4"))

	// 解析 DHCP 租约时间
	dhcpLeaseTime := parseInt(getEnv("NF_DHCP_LEASE_TIME", "86400"), 86400)

	// 解析 ProxyDHCP 模式
	dhcpProxyMode := parseBool(getEnv("NF_DHCP_PROXY_MODE", "false"))

	// 如果未设置 TFTP 服务器，从 ServerAddr 推断
	tftpServer := getEnv("NF_DHCP_TFTP_SERVER", "")
	if tftpServer == "" && serverAddr != "" {
		// 从 serverAddr 中提取 IP 地址
		if idx := strings.Index(serverAddr, ":"); idx > 0 {
			tftpServer = serverAddr[:idx]
		} else {
			tftpServer = serverAddr
		}
	}

	return &Config{
		HTTPAddr:        httpAddr,
		DHCPAddr:        getEnv("NF_DHCP_ADDR", ":67"),
		DHCPInterface:   getEnv("NF_DHCP_INTERFACE", ""),
		DHCPTFTPServer:  tftpServer,
		DHCPProxyMode:   dhcpProxyMode,
		MQTTBroker:      getEnv("NF_MQTT_BROKER", "localhost:1883"),
		MirrorURL:       mirrorURL,
		DBPath:          getEnv("NF_DB_PATH", "/var/lib/nodefoundry/nodes.db"),
		LogLevel:        getEnv("NF_LOG_LEVEL", "info"),
		ServerAddr:      serverAddr,
		DHCPIPPoolStart: getEnv("NF_DHCP_IP_POOL_START", ""),
		DHCPIPPoolEnd:   getEnv("NF_DHCP_IP_POOL_END", ""),
		DHCPNetmask:     getEnv("NF_DHCP_NETMASK", "255.255.255.0"),
		DHCPGateway:     getEnv("NF_DHCP_GATEWAY", ""),
		DHCPDNS:         dhcpDNS,
		DHCPLeaseTime:   dhcpLeaseTime,
	}
}

// parseDNSList 解析 DNS 列表（逗号分隔）
func parseDNSList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parseInt 解析整数，失败返回默认值
func parseInt(s string, defaultVal int) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return defaultVal
}

// parseBool 解析布尔值
func parseBool(s string) bool {
	return strings.ToLower(s) == "true" || s == "1"
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// GetHeartbeatInterval 获取心跳间隔时间
func (c *Config) GetHeartbeatInterval() time.Duration {
	// 默认 60 秒
	return 60 * time.Second
}

// GetIPXESleepInterval 获取 iPXE 等待循环的睡眠时间
func (c *Config) GetIPXESleepInterval() time.Duration {
	// 默认 90 秒
	return 90 * time.Second
}
