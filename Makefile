# Makefile for NodeFoundry

# 变量
BINARY_DIR=bin
SERVER_BINARY=$(BINARY_DIR)/nodefoundry
AGENT_BINARY=$(BINARY_DIR)/nodefoundry-agent
AGENT_ARM64_BINARY=$(BINARY_DIR)/nodefoundry-agent-arm64

# Go 参数
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# 构建标志
LDFLAGS=-ldflags "-s -w"

.PHONY: all build build-server build-agent build-agent-arm64 clean test help

# 默认目标：构建所有
all: build-server build-agent build-agent-arm64

# 构建服务器（当前平台）
build-server:
	@echo "Building NodeFoundry server..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(SERVER_BINARY) ./cmd/nodefoundry
	@echo "Server built: $(SERVER_BINARY)"

# 构建 Agent（当前平台）
build-agent:
	@echo "Building NodeFoundry agent..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(AGENT_BINARY) ./cmd/nodefoundry-agent
	@echo "Agent built: $(AGENT_BINARY)"

# 构建 Agent（ARM64 交叉编译）
build-agent-arm64:
	@echo "Building NodeFoundry agent for ARM64..."
	@mkdir -p $(BINARY_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(AGENT_ARM64_BINARY) ./cmd/nodefoundry-agent
	@echo "Agent ARM64 built: $(AGENT_ARM64_BINARY)"

# 构建所有
build: all

# 清理构建产物
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BINARY_DIR)
	@echo "Cleaned"

# 运行测试
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# 下载依赖
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# 帮助信息
help:
	@echo "Makefile for NodeFoundry"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all               构建所有（服务器 + Agent + Agent ARM64）"
	@echo "  build             构建所有（同 all）"
	@echo "  build-server      构建服务器（当前平台）"
	@echo "  build-agent       构建 Agent（当前平台）"
	@echo "  build-agent-arm64 构建 Agent（ARM64 交叉编译）"
	@echo "  clean             清理构建产物"
	@echo "  test              运行测试"
	@echo "  deps              下载依赖"
	@echo "  help              显示此帮助信息"
