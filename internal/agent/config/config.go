package config

import (
	"os"
	"strconv"
	"time"
)

// Config Agent 配置
type Config struct {
	// MQTT Broker 地址
	MQTTBroker string
	// 日志级别
	LogLevel string
	// MAC 地址（可选，优先使用环境变量）
	MAC string
	// 心跳间隔（秒）
	HeartbeatInterval int
}

// LoadConfig 从环境变量加载配置
func LoadConfig() *Config {
	// 解析心跳间隔，默认 30 秒
	heartbeatInterval := parseInt(getEnv("NF_HEARTBEAT_INTERVAL", "30"), 30)
	if heartbeatInterval < 10 {
		heartbeatInterval = 10 // 最小 10 秒
	}

	return &Config{
		MQTTBroker:         getEnv("NF_MQTT_BROKER", "localhost:1883"),
		LogLevel:           getEnv("NF_LOG_LEVEL", "info"),
		MAC:                getEnv("NF_MAC", ""),
		HeartbeatInterval:  heartbeatInterval,
	}
}

// GetHeartbeatInterval 获取心跳间隔时间
func (c *Config) GetHeartbeatInterval() time.Duration {
	return time.Duration(c.HeartbeatInterval) * time.Second
}

// parseInt 解析整数，失败返回默认值
func parseInt(s string, defaultVal int) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return defaultVal
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
