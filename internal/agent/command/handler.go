package command

import (
	"context"
)

// Handler 命令处理器接口
type Handler interface {
	// Name 返回命令名称
	Name() string

	// Execute 执行命令
	// args: 命令参数（可选）
	Execute(ctx context.Context, args map[string]interface{}) error
}
