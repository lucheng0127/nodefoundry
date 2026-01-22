package server

import (
	"os"
	"time"
)

// Config 服务配置
type Config struct {
	// HTTP 服务地址
	HTTPAddr string
	// DHCP 服务地址
	DHCPAddr string
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

	return &Config{
		HTTPAddr:   httpAddr,
		DHCPAddr:   getEnv("NF_DHCP_ADDR", ":67"),
		MQTTBroker: getEnv("NF_MQTT_BROKER", "localhost:1883"),
		MirrorURL:  mirrorURL,
		DBPath:     getEnv("NF_DB_PATH", "/var/lib/nodefoundry/nodes.db"),
		LogLevel:   getEnv("NF_LOG_LEVEL", "info"),
		ServerAddr: serverAddr,
	}
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
