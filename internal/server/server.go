package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/lucheng0127/nodefoundry/internal/api"
	"github.com/lucheng0127/nodefoundry/internal/db"
	"github.com/lucheng0127/nodefoundry/internal/dhcp"
	"github.com/lucheng0127/nodefoundry/internal/ipxe"
	"github.com/lucheng0127/nodefoundry/internal/mqtt"
)

// Server 服务器
type Server struct {
	config     *Config
	httpServer *http.Server
	dhcpServer *dhcp.DHCPServer
	mqttClient *mqtt.Client
	repo       db.NodeRepository
	db         *bbolt.DB
	logger     *zap.Logger
}

// NewServer 创建服务器
func NewServer(config *Config, logger *zap.Logger) (*Server, error) {
	// 初始化数据库
	boltDB, err := db.InitializeDB(config.DBPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// 创建 repository
	repo := db.NewBoltNodeRepository(boltDB, logger)

	// 创建 iPXE 生成器
	ipxeGen := ipxe.NewGenerator(config.ServerAddr, config.MirrorURL, repo, logger)
	preseedGen := ipxe.NewPreseedGenerator(config.ServerAddr, config.MirrorURL, repo, logger)

	// 创建 API handler
	apiHandler := api.NewHandler(repo, ipxeGen, preseedGen, logger)

	// 创建 HTTP 服务器
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	apiHandler.RegisterRoutes(router)

	httpServer := &http.Server{
		Addr:    config.HTTPAddr,
		Handler: router,
	}

	// 创建 DHCP 服务器
	dhcpServer := dhcp.NewDHCPServer(config.DHCPAddr, config.DHCPInterface, repo, logger)

	// 配置 IP 池（如果设置）
	if config.DHCPIPPoolStart != "" && config.DHCPIPPoolEnd != "" {
		ipManager, err := dhcp.NewIPManager(
			config.DHCPIPPoolStart,
			config.DHCPIPPoolEnd,
			config.DHCPNetmask,
			config.DHCPGateway,
			config.DHCPDNS,
			config.DHCPLeaseTime,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create IP manager: %w", err)
		}
		dhcpServer.SetIPManager(ipManager)
	}

	// 设置 TFTP 服务器
	if config.DHCPTFTPServer != "" {
		dhcpServer.SetTFTPServer(config.DHCPTFTPServer)
	}

	// 设置 ProxyDHCP 模式
	if config.DHCPProxyMode {
		dhcpServer.SetProxyMode(true)
	}

	// 创建 MQTT 客户端
	mqttClient := mqtt.NewClient(config.MQTTBroker, repo, logger)

	return &Server{
		config:     config,
		httpServer: httpServer,
		dhcpServer: dhcpServer,
		mqttClient: mqttClient,
		repo:       repo,
		db:         boltDB,
		logger:     logger,
	}, nil
}

// Start 启动所有服务
func (s *Server) Start(ctx context.Context) error {
	// 创建 errgroup 用于管理 goroutine
	group, ctx := errgroup.WithContext(ctx)

	// 启动 DHCP 服务器
	group.Go(func() error {
		if err := s.dhcpServer.Start(ctx); err != nil {
			return fmt.Errorf("DHCP server error: %w", err)
		}
		return nil
	})

	// 启动 MQTT 客户端
	group.Go(func() error {
		if err := s.mqttClient.Start(ctx); err != nil {
			return fmt.Errorf("MQTT client error: %w", err)
		}
		return nil
	})

	// 启动 HTTP 服务器
	group.Go(func() error {
		s.logger.Info("HTTP server starting", zap.String("addr", s.config.HTTPAddr))
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("HTTP server error: %w", err)
		}
		return nil
	})

	// 等待所有服务完成或出错
	if err := group.Wait(); err != nil {
		s.logger.Error("server error", zap.Error(err))
		return err
	}

	return nil
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("server shutting down")

	// 关闭 HTTP 服务器
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("failed to shutdown HTTP server", zap.Error(err))
	}

	// 关闭数据库
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			s.logger.Error("failed to close database", zap.Error(err))
		}
	}

	s.logger.Info("server shutdown complete")
	return nil
}

// Run 运行服务器（带信号处理）
func (s *Server) Run() error {
	// 创建 context 用于取消
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 启动服务器（在 goroutine 中）
	errChan := make(chan error, 1)
	go func() {
		errChan <- s.Start(ctx)
	}()

	// 等待信号或错误
	select {
	case <-sigChan:
		s.logger.Info("received shutdown signal")
		cancel()
	case err := <-errChan:
		return err
	}

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	return s.Shutdown(shutdownCtx)
}
