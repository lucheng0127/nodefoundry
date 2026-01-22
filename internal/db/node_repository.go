package db

import (
	"context"

	"github.com/lucheng0127/nodefoundry/internal/model"
)

// NodeRepository 定义节点存储接口
type NodeRepository interface {
	// Save 保存或更新节点
	Save(ctx context.Context, node *model.Node) error

	// FindByMAC 根据 MAC 地址查找节点
	FindByMAC(ctx context.Context, mac string) (*model.Node, error)

	// List 列出所有节点
	List(ctx context.Context) ([]*model.Node, error)

	// ListByStatus 按状态筛选节点
	ListByStatus(ctx context.Context, status string) ([]*model.Node, error)

	// UpdateStatus 更新节点状态（带转换验证）
	UpdateStatus(ctx context.Context, mac string, status string) error

	// Delete 删除节点
	Delete(ctx context.Context, mac string) error
}

// ErrNodeNotFound 节点不存在错误
type ErrNodeNotFound struct {
	MAC string
}

func (e *ErrNodeNotFound) Error() string {
	return "node not found"
}

// ErrNodeAlreadyExists 节点已存在错误
type ErrNodeAlreadyExists struct {
	MAC string
}

func (e *ErrNodeAlreadyExists) Error() string {
	return "node already exists"
}

// ErrInvalidStatusTransition 非法状态转换错误
type ErrInvalidStatusTransition struct {
	From string
	To   string
}

func (e *ErrInvalidStatusTransition) Error() string {
	return "invalid status transition"
}
