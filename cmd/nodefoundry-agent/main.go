package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/lucheng0127/nodefoundry/internal/agent"
	"github.com/lucheng0127/nodefoundry/internal/agent/command"
	"github.com/lucheng0127/nodefoundry/internal/agent/config"
	"github.com/lucheng0127/nodefoundry/internal/agent/info"
)

func main() {
	// 加载配置
	cfg := config.LoadConfig()

	// 初始化日志
	logger, err := initLogger(cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	logger.Info("starting NodeFoundry Agent",
		zap.String("mqtt_broker", cfg.MQTTBroker),
		zap.String("log_level", cfg.LogLevel),
		zap.Int("heartbeat_interval", cfg.HeartbeatInterval),
	)

	// 获取 MAC 地址
	mac, err := agent.GetMACAddress(cfg.MAC)
	if err != nil {
		logger.Error("failed to get MAC address", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("agent MAC address", zap.String("mac", mac), zap.String("formatted", agent.FormatMAC(mac)))

	// 创建上下文和取消函数
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建命令分发器
	dispatcher := agent.NewDispatcher(logger)
	dispatcher.Register(command.NewRebootCommand())

	// 创建 MQTT 客户端
	mqttClient := agent.NewMQTTClient(cfg.MQTTBroker, mac, logger, func(payload []byte) {
		if err := dispatcher.Dispatch(ctx, payload); err != nil {
			logger.Error("command dispatch failed", zap.Error(err))
		}
	})

	// 连接 MQTT
	if err := mqttClient.Connect(); err != nil {
		logger.Error("failed to connect to MQTT broker", zap.Error(err))
		os.Exit(1)
	}
	defer mqttClient.Disconnect()

	// 发布初始状态
	if err := publishStatus(mqttClient, mac, logger); err != nil {
		logger.Error("failed to publish initial status", zap.Error(err))
	}

	// 启动心跳定时器
	heartbeatTicker := time.NewTicker(cfg.GetHeartbeatInterval())
	defer heartbeatTicker.Stop()

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("agent started", zap.String("mac", mac))

	// 主循环
	for {
		select {
		case <-heartbeatTicker.C:
			// 心跳定时器触发，发布状态
			if err := publishStatus(mqttClient, mac, logger); err != nil {
				logger.Error("failed to publish heartbeat", zap.Error(err))
			}

		case sig := <-sigChan:
			logger.Info("received signal, shutting down", zap.String("signal", sig.String()))
			cancel()
			return

		case <-ctx.Done():
			logger.Info("context cancelled, shutting down")
			return
		}
	}
}

// initLogger 初始化日志
func initLogger(level string) (*zap.Logger, error) {
	var zapLevel zap.AtomicLevel
	switch level {
	case "debug":
		zapLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapLevel = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapLevel = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		zapLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zapLevel

	return cfg.Build()
}

// publishStatus 发布节点状态
func publishStatus(mqttClient *agent.MQTTClient, mac string, logger *zap.Logger) error {
	// 收集节点信息
	ip := info.GetIP()
	hostname := info.GetHostname()
	uptime := info.GetUptime()

	logger.Debug("collecting node info",
		zap.String("mac", mac),
		zap.String("ip", ip),
		zap.String("hostname", hostname),
		zap.Int64("uptime", uptime),
	)

	// 发布状态
	return mqttClient.PublishStatus("installed", ip, hostname, uptime)
}
