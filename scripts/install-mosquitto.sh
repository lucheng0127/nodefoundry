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

    # 安装 iptables-persistent 以保存规则
    if ! dpkg -l | grep -q iptables-persistent; then
        info "安装 iptables-persistent..."
        export DEBIAN_FRONTEND=noninteractive
        apt-get install -y iptables-persistent
    fi

    # 添加 iptables 规则
    info "添加 iptables 规则..."

    # 允许 MQTT (TCP 1883)
    iptables -A INPUT -p tcp --dport 1883 -j ACCEPT

    # 允许已建立的连接
    iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

    # 允许本地回环
    iptables -A INPUT -i lo -j ACCEPT

    # 保存规则
    if command -v netfilter-persistent &> /dev/null; then
        netfilter-persistent save
    else
        iptables-save > /etc/iptables/rules.v4
    fi

    info "iptables 防火墙规则已添加并保存"
    warn "请确保系统已安装 iptables-persistent 以使规则重启后生效"
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
