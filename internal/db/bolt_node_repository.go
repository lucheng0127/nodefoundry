package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/lucheng0127/nodefoundry/internal/model"
)

// Bucket 名称
const (
	BUCKET_NODES = "nodes"
)

// BoltNodeRepository bbolt 实现的 NodeRepository
type BoltNodeRepository struct {
	db     *bbolt.DB
	logger *zap.Logger
}

// NewBoltNodeRepository 创建 BoltNodeRepository
func NewBoltNodeRepository(db *bbolt.DB, logger *zap.Logger) *BoltNodeRepository {
	repo := &BoltNodeRepository{
		db:     db,
		logger: logger,
	}

	// 初始化 bucket
	if err := repo.initBucket(); err != nil {
		logger.Error("failed to initialize bucket", zap.Error(err))
	}

	return repo
}

// initBucket 初始化 bucket
func (r *BoltNodeRepository) initBucket() error {
	return r.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BUCKET_NODES))
		return err
	})
}

// Save 保存或更新节点
func (r *BoltNodeRepository) Save(ctx context.Context, node *model.Node) error {
	if err := node.Validate(); err != nil {
		return err
	}

	mac := model.NormalizeMAC(node.MAC)

	return r.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_NODES))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		// 检查是否已存在
		existing := b.Get([]byte(mac))
		now := time.Now()

		if existing != nil {
			// 更新现有节点，保持 CreatedAt 不变
			var existingNode model.Node
			if err := json.Unmarshal(existing, &existingNode); err != nil {
				return err
			}
			node.CreatedAt = existingNode.CreatedAt
		} else {
			// 新节点
			node.CreatedAt = now
		}

		node.UpdatedAt = now
		node.MAC = mac

		data, err := json.Marshal(node)
		if err != nil {
			return err
		}

		return b.Put([]byte(mac), data)
	})
}

// FindByMAC 根据 MAC 地址查找节点
func (r *BoltNodeRepository) FindByMAC(ctx context.Context, mac string) (*model.Node, error) {
	mac = model.NormalizeMAC(mac)

	var node *model.Node
	err := r.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_NODES))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		data := b.Get([]byte(mac))
		if data == nil {
			return &ErrNodeNotFound{MAC: mac}
		}

		var n model.Node
		if err := json.Unmarshal(data, &n); err != nil {
			return err
		}

		node = &n
		return nil
	})

	if err != nil {
		return nil, err
	}

	return node, nil
}

// List 列出所有节点
func (r *BoltNodeRepository) List(ctx context.Context) ([]*model.Node, error) {
	var nodes []*model.Node

	err := r.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_NODES))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		return b.ForEach(func(k, v []byte) error {
			var node model.Node
			if err := json.Unmarshal(v, &node); err != nil {
				return err
			}
			nodes = append(nodes, &node)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	// 按 CreatedAt 降序排列
	sortNodes(nodes)

	return nodes, nil
}

// ListByStatus 按状态筛选节点
func (r *BoltNodeRepository) ListByStatus(ctx context.Context, status string) ([]*model.Node, error) {
	allNodes, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []*model.Node
	for _, node := range allNodes {
		if node.Status == status {
			result = append(result, node)
		}
	}

	return result, nil
}

// UpdateStatus 更新节点状态（带转换验证）
func (r *BoltNodeRepository) UpdateStatus(ctx context.Context, mac string, status string) error {
	mac = model.NormalizeMAC(mac)

	return r.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_NODES))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		data := b.Get([]byte(mac))
		if data == nil {
			return &ErrNodeNotFound{MAC: mac}
		}

		var node model.Node
		if err := json.Unmarshal(data, &node); err != nil {
			return err
		}

		// 检查状态转换是否合法
		if err := node.CanTransitionTo(status); err != nil {
			return &ErrInvalidStatusTransition{From: node.Status, To: status}
		}

		node.Status = status
		node.UpdatedAt = time.Now()

		updatedData, err := json.Marshal(node)
		if err != nil {
			return err
		}

		return b.Put([]byte(mac), updatedData)
	})
}

// Delete 删除节点
func (r *BoltNodeRepository) Delete(ctx context.Context, mac string) error {
	mac = model.NormalizeMAC(mac)

	return r.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_NODES))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}

		data := b.Get([]byte(mac))
		if data == nil {
			return &ErrNodeNotFound{MAC: mac}
		}

		return b.Delete([]byte(mac))
	})
}

// sortNodes 按 CreatedAt 降序排列节点
func sortNodes(nodes []*model.Node) {
	// 使用简单的冒泡排序（MVP 阶段节点数量不多）
	n := len(nodes)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if nodes[j].CreatedAt.Before(nodes[j+1].CreatedAt) {
				nodes[j], nodes[j+1] = nodes[j+1], nodes[j]
			}
		}
	}
}

// InitializeDB 初始化数据库
func InitializeDB(dbPath string, logger *zap.Logger) (*bbolt.DB, error) {
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 创建 bucket
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BUCKET_NODES))
		return err
	})

	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	logger.Info("database initialized", zap.String("path", dbPath))
	return db, nil
}
