package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/lucheng0127/nodefoundry/internal/server"
)

func main() {
	// 加载配置
	config := server.LoadConfig()

	// 初始化 logger
	logger := newLogger(config.LogLevel)

	// 创建服务器
	srv, err := server.NewServer(config, logger)
	if err != nil {
		logger.Fatal("failed to create server", zap.Error(err))
	}

	// 运行服务器
	if err := srv.Run(); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}

	logger.Info("server exited")
}

// newLogger 创建新的 logger
func newLogger(logLevel string) *zap.Logger {
	var level zapcore.Level
	switch logLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	// 开发环境配置
	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	config.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}

	return logger
}
