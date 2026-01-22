#!/bin/bash
# Mosquitto MQTT Broker 安装脚本
# 用于在 Debian/Ubuntu 系统上安装和配置 Mosquitto

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

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

# 安装 Mosquitto
install_mosquitto() {
    info "安装 Mosquitto MQTT Broker..."

    apt-get update
    apt-get install -y mosquitto mosquitto-clients

    info "Mosquitto 已安装"
}

# 配置 Mosquitto
configure_mosquitto() {
    info "配置 Mosquitto..."

    # 备份原配置
    if [[ -f /etc/mosquitto/mosquitto.conf ]]; then
        cp /etc/mosquitto/mosquitto.conf /etc/mosquitto/mosquit.conf.bak
        info "已备份原配置文件"
    fi

    # 创建新配置
    cat > /etc/mosquitto/mosquitto.conf << 'EOF'
# Mosquitto 配置文件
# 用于 NodeFoundry

# 监听设置
listener 1883
protocol mqtt
allow_anonymous true

# 持久化
persistence true
persistence_location /var/lib/mosquitto/
autosave_interval 1800

# 日志
log_dest file /var/log/mosquitto/mosquitto.log
log_type error
log_type warning
log_type notice
log_type information

# 连接设置
max_connections -1
max_queued_messages 1000

# 消息大小限制
message_size_limit 0
EOF

    # 创建日志目录
    mkdir -p /var/log/mosquitto
    chown mosquitto:mosquitto /var/log/mosquitto

    info "Mosquitto 配置完成"
}

# 启动 Mosquitto
start_mosquitto() {
    info "启动 Mosquitto 服务..."

    systemctl enable mosquitto
    systemctl restart mosquitto

    sleep 2

    if systemctl is-active --quiet mosquitto; then
        info "Mosquitto 服务已启动"
    else
        error "Mosquitto 服务启动失败"
    fi
}

# 配置防火墙
configure_firewall() {
    info "配置防火墙..."

    if command -v ufw &> /dev/null; then
        ufw allow 1883/tcp comment "Mosquitto MQTT"
        info "UFW 防火墙规则已添加"
    elif command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-service=mosquitto
        firewall-cmd --reload
        info "firewalld 防火墙规则已添加"
    else
        warn "未检测到防火墙，请手动配置"
    fi
}

# 测试连接
test_connection() {
    info "测试 Mosquitto 连接..."

    if command -v mosquitto_pub &> /dev/null; then
        # 订阅测试（后台运行）
        timeout 3 mosquitto_sub -h localhost -t "test/topic" -v &
        SUB_PID=$!

        sleep 1

        # 发布测试消息
        mosquitto_pub -h localhost -t "test/topic" -m "test message"

        wait $SUB_PID 2>/dev/null

        info "Mosquitto 连接测试成功"
    else
        warn "未找到 mosquitto_pub，跳过连接测试"
    fi
}

# 显示完成信息
show_completion() {
    info ""
    info "========================================"
    info "  Mosquitto 安装完成！"
    info "========================================"
    info ""
    info "服务管理命令:"
    info "  启动服务: systemctl start mosquitto"
    info "  停止服务: systemctl stop mosquitto"
    info "  重启服务: systemctl restart mosquitto"
    info "  查看状态: systemctl status mosquitto"
    info "  查看日志: journalctl -u mosquitto -f"
    info "  查看日志: tail -f /var/log/mosquitto/mosquitto.log"
    info ""
    info "MQTT 连接信息:"
    info "  地址: localhost:1883"
    info "  NodeFoundry 订阅: node/+/status"
    info ""
}

# 主函数
main() {
    info "开始安装 Mosquitto MQTT Broker..."

    check_root
    install_mosquitto
    configure_mosquitto
    start_mosquitto
    configure_firewall
    test_connection
    show_completion

    info "安装完成！"
}

# 运行主函数
main "$@"
