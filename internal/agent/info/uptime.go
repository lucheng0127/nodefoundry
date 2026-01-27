package info

import (
	"os"
	"strconv"
	"strings"
)

// GetUptime 获取系统运行时长（秒）
func GetUptime() int64 {
	// 读取 /proc/uptime
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}

	// 文件格式: "uptime.idle秒"
	// 示例: "12345.67 98765.43"
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}

	// 解析运行时长（第一个字段）
	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}

	// 转换为秒（向上取整）
	return int64(uptime + 0.5)
}
