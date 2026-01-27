package command

import (
	"context"
	"fmt"
	"os/exec"
)

// RebootCommand 重启命令
type RebootCommand struct{}

// NewRebootCommand 创建重启命令
func NewRebootCommand() *RebootCommand {
	return &RebootCommand{}
}

// Name 返回命令名称
func (c *RebootCommand) Name() string {
	return "reboot"
}

// Execute 执行重启命令
func (c *RebootCommand) Execute(ctx context.Context, args map[string]interface{}) error {
	// 使用 systemctl reboot 命令
	// 这是最安全的重启方式，会正确处理 systemd 环境
	cmd := exec.Command("systemctl", "reboot")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to execute reboot: %w", err)
	}

	// 命令已启动，系统即将重启
	return nil
}
