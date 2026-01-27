package api

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/lucheng0127/nodefoundry/internal/db"
	"github.com/lucheng0127/nodefoundry/internal/ipxe"
	"github.com/lucheng0127/nodefoundry/internal/model"
)

// Handler API 处理器
type Handler struct {
	repo           db.NodeRepository
	ipxeGen        *ipxe.Generator
	preseedGen     *ipxe.PreseedGenerator
	logger         *zap.Logger
	startTime      time.Time
}

// NewHandler 创建 API 处理器
func NewHandler(
	repo db.NodeRepository,
	ipxeGen *ipxe.Generator,
	preseedGen *ipxe.PreseedGenerator,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		repo:       repo,
		ipxeGen:    ipxeGen,
		preseedGen: preseedGen,
		logger:     logger,
		startTime:  time.Now(),
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// API v1
	v1 := r.Group("/api/v1")
	{
		nodes := v1.Group("/nodes")
		{
			nodes.GET("", h.ListNodes)
			nodes.GET("/:mac", h.GetNode)
			nodes.POST("", h.RegisterNode)
			nodes.PUT("/:mac", h.UpdateNode)
		}
	}

	// iPXE 端点
	r.GET("/boot/:mac/boot.ipxe", h.GetBootScript)

	// preseed 端点
	r.GET("/preseed/:mac/preseed.cfg", h.GetPreseed)

	// Agent 下载端点
	r.GET("/agent/nodefoundry-agent", h.GetAgentBinary)
	r.GET("/agent/nodefoundry-agent.service", h.GetAgentServiceFile)

	// 健康检查
	r.GET("/health", h.HealthCheck)
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error"`
}

// errorResponse 返回错误响应
func errorResponse(c *gin.Context, code int, message string) {
	c.JSON(code, ErrorResponse{Error: message})
}

// RegisterNodeRequest 注册节点请求
type RegisterNodeRequest struct {
	MAC string `json:"mac" binding:"required"`
	IP  string `json:"ip,omitempty"`
}

// UpdateNodeRequest 更新节点请求
type UpdateNodeRequest struct {
	Action string `json:"action" binding:"required"`
}

// ListNodes 列出所有节点
func (h *Handler) ListNodes(c *gin.Context) {
	nodes, err := h.repo.List(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to list nodes", zap.Error(err))
		errorResponse(c, http.StatusInternalServerError, "failed to list nodes")
		return
	}

	c.JSON(http.StatusOK, nodes)
}

// GetNode 获取单个节点
func (h *Handler) GetNode(c *gin.Context) {
	mac := c.Param("mac")

	node, err := h.repo.FindByMAC(c.Request.Context(), mac)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "node not found")
		return
	}

	c.JSON(http.StatusOK, node)
}

// RegisterNode 手动注册节点
func (h *Handler) RegisterNode(c *gin.Context) {
	var req RegisterNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}

	// 验证 MAC 地址格式
	if !model.IsValidMAC(req.MAC) {
		errorResponse(c, http.StatusBadRequest, "invalid MAC address format")
		return
	}

	// 检查节点是否已存在
	normalizedMAC := model.NormalizeMAC(req.MAC)
	_, err := h.repo.FindByMAC(c.Request.Context(), normalizedMAC)
	if err == nil {
		errorResponse(c, http.StatusConflict, "node already exists")
		return
	}

	// 创建新节点
	node, err := model.NewNode(normalizedMAC, model.STATE_DISCOVERED)
	if err != nil {
		h.logger.Error("failed to create node", zap.Error(err))
		errorResponse(c, http.StatusInternalServerError, "failed to create node")
		return
	}

	if req.IP != "" {
		node.IP = req.IP
	}

	// 保存节点
	if err := h.repo.Save(c.Request.Context(), node); err != nil {
		h.logger.Error("failed to save node", zap.Error(err))
		errorResponse(c, http.StatusInternalServerError, "failed to save node")
		return
	}

	c.Header("Location", "/api/v1/nodes/"+normalizedMAC)
	c.JSON(http.StatusCreated, node)
}

// UpdateNode 更新节点（支持 action: install）
func (h *Handler) UpdateNode(c *gin.Context) {
	mac := c.Param("mac")

	var req UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}

	node, err := h.repo.FindByMAC(c.Request.Context(), mac)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "node not found")
		return
	}

	switch req.Action {
	case "install":
		// MVP: 仅支持 discovered → installing
		if node.Status != model.STATE_DISCOVERED {
			errorResponse(c, http.StatusBadRequest,
				fmt.Sprintf("cannot install node with status '%s', only 'discovered' nodes can be installed", node.Status))
			return
		}

		// 更新状态为 installing
		if err := h.repo.UpdateStatus(c.Request.Context(), mac, model.STATE_INSTALLING); err != nil {
			h.logger.Error("failed to update node status",
				zap.String("mac", mac),
				zap.Error(err),
			)
			errorResponse(c, http.StatusInternalServerError, "failed to update node status")
			return
		}

		// 获取更新后的节点
		node, _ = h.repo.FindByMAC(c.Request.Context(), mac)
		c.JSON(http.StatusOK, node)

	default:
		errorResponse(c, http.StatusBadRequest, "unknown action")
	}
}

// GetBootScript 获取 iPXE 引导脚本
func (h *Handler) GetBootScript(c *gin.Context) {
	mac := c.Param("mac")

	script, err := h.ipxeGen.GenerateByStatus(c.Request.Context(), mac)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "node not found")
		return
	}

	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, script)
}

// GetPreseed 获取 preseed 配置文件
func (h *Handler) GetPreseed(c *gin.Context) {
	mac := c.Param("mac")

	// 获取查询参数（网络配置）
	query := c.Request.URL.Query()

	preseed, err := h.preseedGen.GenerateWithQuery(c.Request.Context(), mac, query)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "node not found")
		return
	}

	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, preseed)
}

// GetAgentBinary 获取 agent 二进制文件
func (h *Handler) GetAgentBinary(c *gin.Context) {
	// 返回 ARM64 架构的 Agent 二进制文件
	// 路径相对于项目根目录
	agentPath := "bin/nodefoundry-agent-arm64"

	// 检查文件是否存在
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		h.logger.Error("agent binary not found", zap.String("path", agentPath))
		errorResponse(c, http.StatusNotFound, "agent binary not found")
		return
	}

	// 读取文件
	data, err := os.ReadFile(agentPath)
	if err != nil {
		h.logger.Error("failed to read agent binary", zap.Error(err))
		errorResponse(c, http.StatusInternalServerError, "failed to read agent binary")
		return
	}

	// 设置响应头
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=nodefoundry-agent")
	c.Data(http.StatusOK, "application/octet-stream", data)

	h.logger.Debug("agent binary downloaded", zap.Int("size", len(data)))
}

// GetAgentServiceFile 获取 systemd 服务文件
func (h *Handler) GetAgentServiceFile(c *gin.Context) {
	serviceFile := `[Unit]
Description=NodeFoundry Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nodefoundry-agent
Restart=always
RestartSec=10
EnvironmentFile=/etc/default/nodefoundry-agent

[Install]
WantedBy=multi-user.target
`

	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, serviceFile)
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status string `json:"status"`
	Uptime string `json:"uptime"`
}

// HealthCheck 健康检查
func (h *Handler) HealthCheck(c *gin.Context) {
	uptime := time.Since(h.startTime)
	c.JSON(http.StatusOK, HealthResponse{
		Status: "ok",
		Uptime: uptime.String(),
	})
}
