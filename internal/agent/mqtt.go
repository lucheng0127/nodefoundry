package agent

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

// MQTTClient MQTT 客户端
type MQTTClient struct {
	broker      string
	client      mqtt.Client
	mac         string
	logger      *zap.Logger
	connectChan chan bool
	onCommand   func([]byte)
}

// NewMQTTClient 创建 MQTT 客户端
func NewMQTTClient(broker, mac string, logger *zap.Logger, onCommand func([]byte)) *MQTTClient {
	return &MQTTClient{
		broker:      broker,
		mac:         mac,
		logger:      logger,
		connectChan: make(chan bool, 1),
		onCommand:   onCommand,
	}
}

// Connect 连接到 MQTT Broker
func (m *MQTTClient) Connect() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(m.broker)
	opts.SetClientID(fmt.Sprintf("nodefoundry-agent-%s", m.mac))
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.SetOnConnectHandler(m.onConnect)
	opts.SetConnectionLostHandler(m.onConnectionLost)

	m.client = mqtt.NewClient(opts)

	// 连接到 Broker
	if token := m.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	// 等待连接确认
	select {
	case <-m.connectChan:
		m.logger.Info("MQTT client connected", zap.String("broker", m.broker))
	case <-time.After(30 * time.Second):
		return fmt.Errorf("MQTT connection timeout")
	}

	return nil
}

// onConnect 连接成功回调
func (m *MQTTClient) onConnect(client mqtt.Client) {
	// 订阅命令主题
	commandTopic := fmt.Sprintf("node/%s/command", m.mac)
	if token := client.Subscribe(commandTopic, 0, m.onCommandMessage); token.Wait() && token.Error() != nil {
		m.logger.Error("failed to subscribe to command topic", zap.Error(token.Error()))
		return
	}

	m.logger.Info("subscribed to command topic", zap.String("topic", commandTopic))

	select {
	case m.connectChan <- true:
	default:
	}
}

// onConnectionLost 连接丢失回调
func (m *MQTTClient) onConnectionLost(client mqtt.Client, err error) {
	m.logger.Warn("MQTT connection lost", zap.Error(err))
}

// onCommandMessage 处理命令消息
func (m *MQTTClient) onCommandMessage(client mqtt.Client, msg mqtt.Message) {
	payload := msg.Payload()

	m.logger.Debug("received command message",
		zap.String("topic", msg.Topic()),
		zap.String("payload", string(payload)),
	)

	// 调用命令处理回调
	if m.onCommand != nil {
		go m.onCommand(payload)
	}
}

// PublishStatus 发布状态消息
func (m *MQTTClient) PublishStatus(status string, ip, hostname string, uptime int64) error {
	// 构建状态消息
	payload := fmt.Sprintf(`{"status":"%s","ip":"%s","hostname":"%s","uptime":%d,"timestamp":"%s"}`,
		status, ip, hostname, uptime, time.Now().Format(time.RFC3339))

	// 发布到状态主题
	topic := fmt.Sprintf("node/%s/status", m.mac)
	token := m.client.Publish(topic, 0, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish status: %w", token.Error())
	}

	m.logger.Debug("status published",
		zap.String("topic", topic),
		zap.String("status", status),
		zap.String("ip", ip),
		zap.String("hostname", hostname),
		zap.Int64("uptime", uptime),
	)

	return nil
}

// Disconnect 断开连接
func (m *MQTTClient) Disconnect() {
	if m.client != nil && m.client.IsConnected() {
		m.logger.Info("disconnecting MQTT client")
		m.client.Disconnect(250)
	}
}
