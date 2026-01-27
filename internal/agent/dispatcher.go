package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/lucheng0127/nodefoundry/internal/agent/command"
)

// CommandMessage 命令消息结构
type CommandMessage struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

// Dispatcher 命令分发器
type Dispatcher struct {
	handlers map[string]command.Handler
	logger   *zap.Logger
}

// NewDispatcher 创建命令分发器
func NewDispatcher(logger *zap.Logger) *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string]command.Handler),
		logger:   logger,
	}
}

// Register 注册命令处理器
func (d *Dispatcher) Register(handler command.Handler) {
	d.handlers[handler.Name()] = handler
	d.logger.Info("command handler registered", zap.String("command", handler.Name()))
}

// Dispatch 分发并执行命令
func (d *Dispatcher) Dispatch(ctx context.Context, payload []byte) error {
	// 解析命令消息
	var msg CommandMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("failed to parse command message: %w", err)
	}

	// 查找处理器
	handler, exists := d.handlers[msg.Command]
	if !exists {
		return fmt.Errorf("unknown command: %s", msg.Command)
	}

	d.logger.Info("executing command",
		zap.String("command", msg.Command),
		zap.Any("args", msg.Args),
	)

	// 执行命令
	if err := handler.Execute(ctx, msg.Args); err != nil {
		d.logger.Error("command execution failed",
			zap.String("command", msg.Command),
			zap.Error(err),
		)
		return fmt.Errorf("command %s failed: %w", msg.Command, err)
	}

	d.logger.Info("command executed successfully", zap.String("command", msg.Command))
	return nil
}
