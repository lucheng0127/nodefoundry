#!/bin/bash
# NodeFoundry 部署脚本
# 用于在 Debian/Ubuntu 系统上部署 NodeFoundry 服务

set -e

# 配置变量
INSTALL_DIR="/opt/nodefoundry"
BINARY_NAME="nodefoundry"
SERVICE_FILE="/etc/systemd/system/nodefoundry.service"
DB_DIR="/var/lib/nodefoundry"
CONFIG_DIR="/etc/nodefoundry"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# 检查是否为 root 用户
check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "此脚本需要 root 权限运行"
    fi
}

# 检查系统
check_system() {
    info "检查系统..."

    if [[ ! -f /etc/os-release ]]; then
        error "无法检测操作系统"
    fi

    . /etc/os-release
    info "检测到操作系统: $PRETTY_NAME"

    if [[ "$ID" != "debian" && "$ID" != "ubuntu" ]]; then
        warn "此脚本主要针对 Debian/Ubuntu 系统"
    fi
}

# 安装依赖
install_dependencies() {
    info "安装依赖..."

    apt-get update
    apt-get install -y mosquitto wget curl
}

# 创建用户和目录
create_directories() {
    info "创建目录和用户..."

    # 创建用户
    if ! id -u nodefoundry &>/dev/null; then
        useradd -r -s /bin/false -d $INSTALL_DIR nodefoundry
        info "创建用户: nodefoundry"
    fi

    # 创建目录
    mkdir -p $INSTALL_DIR
    mkdir -p $DB_DIR
    mkdir -p $CONFIG_DIR

    # 设置权限
    chown -R nodefoundry:nodefoundry $INSTALL_DIR
    chown -R nodefoundry:nodefoundry $DB_DIR
}

# 复制二进制文件
install_binary() {
    info "安装二进制文件..."

    if [[ ! -f "./bin/$BINARY_NAME" ]]; then
        error "找不到二进制文件 ./bin/$BINARY_NAME，请先运行 go build"
    fi

    cp ./bin/$BINARY_NAME $INSTALL_DIR/
    chmod +x $INSTALL_DIR/$BINARY_NAME
    chown nodefoundry:nodefoundry $INSTALL_DIR/$BINARY_NAME

    info "二进制文件已安装到 $INSTALL_DIR/$BINARY_NAME"
}

# 安装配置文件
install_config() {
    info "安装配置文件..."

    cat > $CONFIG_DIR/defaults << 'EOF'
# NodeFoundry 默认配置
# 这些值会被 systemd 服务中的环境变量覆盖

NF_HTTP_ADDR=:8080
NF_DHCP_ADDR=:67
NF_MQTT_BROKER=localhost:1883
NF_MIRROR_URL=mirrors.ustc.edu.cn
NF_DB_PATH=/var/lib/nodefoundry/nodes.db
NF_LOG_LEVEL=info
# NF_SERVER_ADDR 会自动推断，建议在 systemd 文件中明确设置
EOF

    chown nodefoundry:nodefoundry $CONFIG_DIR/defaults
}

# 安装 systemd 服务
install_service() {
    info "安装 systemd 服务..."

    cat > $SERVICE_FILE << EOF
[Unit]
Description=NodeFoundry Edge Node Management Server
Documentation=https://github.com/lucheng0127/nodefoundry
After=network.target mosquitto.service
Requires=mosquitto.service

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
EnvironmentFile=$CONFIG_DIR/defaults
ExecStart=$INSTALL_DIR/$BINARY_NAME
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DB_DIR

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    info "systemd 服务已安装"
}

# 配置防火墙
configure_firewall() {
    info "配置防火墙..."

    if command -v ufw &> /dev/null; then
        ufw allow 67/udp comment "DHCP"
        ufw allow 8080/tcp comment "NodeFoundry HTTP"
        info "UFW 防火墙规则已添加"
    elif command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-service=dhcp
        firewall-cmd --permanent --add-port=8080/tcp
        firewall-cmd --reload
        info "firewalld 防火墙规则已添加"
    else
        warn "未检测到防火墙，请手动配置"
    fi
}

# 启动服务
start_service() {
    info "启动 NodeFoundry 服务..."

    systemctl enable nodefoundry
    systemctl start nodefoundry

    sleep 2

    if systemctl is-active --quiet nodefoundry; then
        info "NodeFoundry 服务已启动"
        systemctl status nodefoundry --no-pager
    else
        error "NodeFoundry 服务启动失败"
    fi
}

# 显示完成信息
show_completion() {
    info ""
    info "========================================"
    info "  NodeFoundry 部署完成！"
    info "========================================"
    info ""
    info "服务管理命令:"
    info "  启动服务: systemctl start nodefoundry"
    info "  停止服务: systemctl stop nodefoundry"
    info "  重启服务: systemctl restart nodefoundry"
    info "  查看状态: systemctl status nodefoundry"
    info "  查看日志: journalctl -u nodefoundry -f"
    info ""
    info "配置文件位置: $CONFIG_DIR/defaults"
    info "数据库位置: $DB_DIR/nodes.db"
    info ""
    info "API 端点:"
    info "  GET  http://localhost:8080/api/v1/nodes"
    info "  GET  http://localhost:8080/api/v1/nodes/:mac"
    info "  POST http://localhost:8080/api/v1/nodes"
    info "  PUT  http://localhost:8080/api/v1/nodes/:mac"
    info "  GET  http://localhost:8080/health"
    info ""
    warn "请确保在 $CONFIG_DIR/defaults 中设置 NF_SERVER_ADDR"
    warn "以便节点可以正确连接到此服务器"
    info ""
}

# 主函数
main() {
    info "开始部署 NodeFoundry..."

    check_root
    check_system
    install_dependencies
    create_directories
    install_binary
    install_config
    install_service
    configure_firewall
    start_service
    show_completion

    info "部署完成！"
}

# 运行主函数
main "$@"
