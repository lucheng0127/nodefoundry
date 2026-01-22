package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Node 表示边缘节点
type Node struct {
	MAC           string          `json:"mac"`
	IP            string          `json:"ip,omitempty"`
	Hostname      string          `json:"hostname,omitempty"`
	Status        string          `json:"status"`
	LastHeartbeat time.Time       `json:"last_heartbeat,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	Extra         json.RawMessage `json:"extra,omitempty"`
}

// 状态常量
const (
	STATE_DISCOVERED = "discovered"
	STATE_INSTALLING = "installing"
	STATE_INSTALLED  = "installed"
)

// 所有有效状态
var validStates = map[string]bool{
	STATE_DISCOVERED: true,
	STATE_INSTALLING: true,
	STATE_INSTALLED:  true,
}

// IsValidStatus 验证状态是否有效
func IsValidStatus(status string) bool {
	return validStates[status]
}

// 状态转换规则（MVP：单向，不支持回退或重装）
var stateTransitions = map[string][]string{
	STATE_DISCOVERED: {STATE_INSTALLING},
	STATE_INSTALLING: {STATE_INSTALLED},
	STATE_INSTALLED:  {}, // 已安装状态不能转换
}

// CanTransitionTo 检查状态转换是否合法
// MVP: discovered → installing → installed (单向，不支持回退或重装)
func (n *Node) CanTransitionTo(newStatus string) error {
	// 检查新状态是否有效
	if !IsValidStatus(newStatus) {
		return fmt.Errorf("invalid status: %s", newStatus)
	}

	// 状态未变化
	if n.Status == newStatus {
		return nil
	}

	// 检查是否允许转换
	allowedStates, exists := stateTransitions[n.Status]
	if !exists {
		return fmt.Errorf("unknown current status: %s", n.Status)
	}

	for _, allowed := range allowedStates {
		if allowed == newStatus {
			return nil
		}
	}

	return fmt.Errorf("invalid status transition: %s -> %s", n.Status, newStatus)
}

// Validate 验证节点数据
func (n *Node) Validate() error {
	if n.MAC == "" {
		return errors.New("mac address is required")
	}

	if !IsValidStatus(n.Status) {
		return fmt.Errorf("invalid status: %s", n.Status)
	}

	return nil
}

// NewNode 创建新节点
func NewNode(mac string, status string) (*Node, error) {
	if !IsValidMAC(mac) {
		return nil, errors.New("invalid MAC address format")
	}

	if !IsValidStatus(status) {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	now := time.Now()
	return &Node{
		MAC:       NormalizeMAC(mac),
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// NormalizeMAC 标准化 MAC 地址为小写、无分隔符格式
func NormalizeMAC(mac string) string {
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

// IsValidMAC 验证 MAC 地址格式
func IsValidMAC(mac string) bool {
	normalized := NormalizeMAC(mac)
	return len(normalized) == 12
}
