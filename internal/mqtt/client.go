package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"

	"github.com/lucheng0127/nodefoundry/internal/db"
	"github.com/lucheng0127/nodefoundry/internal/model"
)

// Client MQTT 客户端（仅接收状态）
type Client struct {
	broker       string
	client       mqtt.Client
	repo         db.NodeRepository
	logger       *zap.Logger
	connectChan  chan bool
}

// StatusMessage 状态消息结构
type StatusMessage struct {
	Status   string `json:"status"`
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Uptime   int64  `json:"uptime,omitempty"`
}

// NewClient 创建 MQTT 客户端
func NewClient(broker string, repo db.NodeRepository, logger *zap.Logger) *Client {
	return &Client{
		broker:      broker,
		repo:        repo,
		logger:      logger,
		connectChan: make(chan bool, 1),
	}
}

// Start 启动 MQTT 客户端
func (c *Client) Start(ctx context.Context) error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(c.broker)
	opts.SetClientID("nodefoundry-server")
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetOnConnectHandler(c.onConnect)
	opts.SetConnectionLostHandler(c.onConnectionLost)

	c.client = mqtt.NewClient(opts)

	// 连接到 Broker
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	// 等待连接确认
	select {
	case <-c.connectChan:
		c.logger.Info("MQTT client connected", zap.String("broker", c.broker))
	case <-time.After(30 * time.Second):
		return fmt.Errorf("MQTT connection timeout")
	}

	// 等待 context 取消
	<-ctx.Done()

	c.logger.Info("MQTT client shutting down")
	c.client.Disconnect(250)

	return nil
}

// onConnect 连接成功回调
func (c *Client) onConnect(client mqtt.Client) {
	// 订阅节点状态主题
	topic := "node/+/status"
	if token := client.Subscribe(topic, 0, c.onStatusMessage); token.Wait() && token.Error() != nil {
		c.logger.Error("failed to subscribe to status topic", zap.Error(token.Error()))
		return
	}

	c.logger.Info("subscribed to status topic", zap.String("topic", topic))

	select {
	case c.connectChan <- true:
	default:
	}
}

// onConnectionLost 连接丢失回调
func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	c.logger.Warn("MQTT connection lost", zap.Error(err))
}

// onStatusMessage 处理状态消息
func (c *Client) onStatusMessage(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	c.logger.Debug("received MQTT message",
		zap.String("topic", topic),
		zap.String("payload", string(payload)),
	)

	// 解析 topic: node/{mac}/status
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "node" || parts[2] != "status" {
		c.logger.Warn("invalid topic format", zap.String("topic", topic))
		return
	}

	mac := model.NormalizeMAC(parts[1])

	// 解析 JSON payload
	var statusMsg StatusMessage
	if err := json.Unmarshal(payload, &statusMsg); err != nil {
		c.logger.Error("failed to parse status message",
			zap.String("mac", mac),
			zap.Error(err),
		)
		return
	}

	// 验证状态是否有效
	if !model.IsValidStatus(statusMsg.Status) {
		c.logger.Warn("invalid status in message",
			zap.String("mac", mac),
			zap.String("status", statusMsg.Status),
		)
		return
	}

	// 获取现有节点
	ctx := context.Background()
	node, err := c.repo.FindByMAC(ctx, mac)
	if err != nil {
		c.logger.Warn("received status from unknown node",
			zap.String("mac", mac),
			zap.Error(err),
		)
		return
	}

	// 检查状态转换是否合法
	if err := node.CanTransitionTo(statusMsg.Status); err != nil {
		c.logger.Warn("invalid status transition",
			zap.String("mac", mac),
			zap.String("from", node.Status),
			zap.String("to", statusMsg.Status),
			zap.Error(err),
		)
		// 即使状态转换无效，仍更新心跳时间
		node.LastHeartbeat = time.Now()
		if statusMsg.IP != "" {
			node.IP = statusMsg.IP
		}
		if statusMsg.Hostname != "" {
			node.Hostname = statusMsg.Hostname
		}
		c.repo.Save(ctx, node)
		return
	}

	// 更新节点状态
	node.Status = statusMsg.Status
	node.LastHeartbeat = time.Now()
	if statusMsg.IP != "" {
		node.IP = statusMsg.IP
	}
	if statusMsg.Hostname != "" {
		node.Hostname = statusMsg.Hostname
	}

	if err := c.repo.Save(ctx, node); err != nil {
		c.logger.Error("failed to update node status",
			zap.String("mac", mac),
			zap.Error(err),
		)
		return
	}

	c.logger.Info("node status updated",
		zap.String("mac", mac),
		zap.String("status", statusMsg.Status),
		zap.String("ip", node.IP),
		zap.String("hostname", node.Hostname),
	)
}
