#!/bin/bash
PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:~/bin
export PATH

# Docker镜像转存工具安装脚本
# 项目地址: https://github.com/yedsn/docker-mirror

# 获取网络环境参数
netEnvCn="$1"
echo "网络环境: ${netEnvCn}"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 获取系统架构
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}不支持的系统架构: $ARCH${NC}"
        exit 1
        ;;
esac

# 获取操作系统
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [[ "$OS" != "linux" ]]; then
    echo -e "${RED}此脚本仅支持Linux系统${NC}"
    exit 1
fi

# 检查是否为root用户
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}请以root用户运行此脚本！${NC}"
    exit 1
fi

# 记录开始时间
START_TIME=$(date +%s)

echo -e "${GREEN}===============================================${NC}"
echo -e "${GREEN}    Docker镜像转存工具 安装脚本${NC}"
echo -e "${GREEN}===============================================${NC}"
echo ""
echo -e "${YELLOW}使用方法:${NC}"
echo -e "  默认安装: bash install.sh"
echo -e "  国内环境: bash install.sh cn"
echo ""

# 检查系统信息
echo -e "${YELLOW}检查系统信息...${NC}"
if grep -Eqi "Debian" /etc/issue || grep -Eq "Debian" /etc/*-release; then
    OSNAME='debian'
elif grep -Eqi "Ubuntu" /etc/issue || grep -Eq "Ubuntu" /etc/*-release; then
    OSNAME='ubuntu'
elif grep -Eqi "CentOS" /etc/issue || grep -Eq "CentOS" /etc/*-release; then
    OSNAME='centos'
elif grep -Eqi "AlmaLinux" /etc/issue || grep -Eq "AlmaLinux" /etc/*-release; then
    OSNAME='centos'
elif grep -Eqi "Rocky" /etc/issue || grep -Eq "Rocky" /etc/*-release; then
    OSNAME='centos'
else
    OSNAME='unknown'
fi

echo -e "系统类型: ${GREEN}$OSNAME${NC}"
echo -e "系统架构: ${GREEN}$ARCH${NC}"
echo ""

# 检查是否已安装Docker
echo -e "${YELLOW}检查Docker安装状态...${NC}"
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: 未检测到Docker，请先安装Docker${NC}"
    echo -e "${YELLOW}安装Docker命令：${NC}"
    echo "curl -fsSL https://get.docker.com | bash"
    exit 1
fi
echo -e "${GREEN}Docker已安装: $(docker --version)${NC}"
echo ""

# 开始下载安装
echo -e "${YELLOW}开始下载预编译二进制文件...${NC}"

# 创建临时目录
TEMP_DIR=$(mktemp -d)
cd $TEMP_DIR

# 根据网络环境选择下载源
if [ "$netEnvCn" == "cn" ]; then
    # 使用Gitee国内镜像源
    DOWNLOAD_URL="https://gitee.com/hongxiaojian/docker-mirror/raw/master/docker-mirror"
    echo -e "${YELLOW}使用国内镜像源（Gitee）...${NC}"
else
    # 使用GitHub源
    DOWNLOAD_URL="https://raw.githubusercontent.com/yedsn/docker-mirror/master/docker-mirror"
    echo -e "${YELLOW}使用GitHub源...${NC}"
fi

echo -e "下载地址: ${GREEN}$DOWNLOAD_URL${NC}"
if ! wget -q --show-progress "$DOWNLOAD_URL" -O docker-mirror; then
    echo -e "${RED}下载失败，请检查网络连接${NC}"
    if [ "$netEnvCn" == "cn" ]; then
        echo -e "${YELLOW}尝试使用GitHub备用源...${NC}"
        BACKUP_URL="https://raw.githubusercontent.com/yedsn/docker-mirror/master/docker-mirror"
        if ! wget -q --show-progress "$BACKUP_URL" -O docker-mirror; then
            echo -e "${RED}备用源下载也失败，请检查网络连接${NC}"
            rm -rf $TEMP_DIR
            exit 1
        fi
    else
        echo -e "${YELLOW}尝试使用Gitee备用源...${NC}"
        BACKUP_URL="https://gitee.com/hongxiaojian/docker-mirror/raw/master/docker-mirror"
        if ! wget -q --show-progress "$BACKUP_URL" -O docker-mirror; then
            echo -e "${RED}备用源下载也失败，请检查网络连接${NC}"
            rm -rf $TEMP_DIR
            exit 1
        fi
    fi
fi

# 安装二进制文件
if [ -f "docker-mirror" ]; then
    chmod +x docker-mirror
    cp docker-mirror /usr/bin/
    echo -e "${GREEN}二进制文件安装成功${NC}"
else
    echo -e "${RED}下载失败，未找到docker-mirror文件${NC}"
    rm -rf $TEMP_DIR
    exit 1
fi

# 清理临时文件
cd /
rm -rf $TEMP_DIR

# 验证安装
echo ""
echo -e "${YELLOW}验证安装...${NC}"
if command -v docker-mirror &> /dev/null; then
    echo -e "${GREEN}docker-mirror 安装成功！${NC}"
    echo ""
    docker-mirror help
else
    echo -e "${RED}安装失败，请检查错误信息${NC}"
    exit 1
fi

# 计算安装时间
END_TIME=$(date +%s)
INSTALL_TIME=$(( (END_TIME - START_TIME) / 60 ))

echo ""
echo -e "${GREEN}===============================================${NC}"
echo -e "${GREEN}    安装完成！${NC}"
echo -e "${GREEN}===============================================${NC}"
echo ""
echo -e "${YELLOW}安装信息:${NC}"
echo -e "安装路径: ${GREEN}/usr/bin/docker-mirror${NC}"
echo -e "配置路径: ${GREEN}~/.config/docker-mirror/config.yaml${NC}"
echo -e "安装时间: ${GREEN}$INSTALL_TIME 分钟${NC}"
echo ""
echo -e "${YELLOW}下一步操作:${NC}"
echo -e "1. 运行 ${GREEN}docker-mirror config${NC} 初始化配置"
echo -e "2. 运行 ${GREEN}docker-mirror help${NC} 查看使用帮助"
echo ""
echo -e "${YELLOW}使用示例:${NC}"
echo -e "  docker-mirror pull nginx:latest"
echo -e "  docker-mirror pull-local redis:alpine"
echo -e "  docker-mirror push mysql:8.0"
echo ""
echo -e "${YELLOW}安装脚本使用方法:${NC}"
echo -e "  海外环境: curl -fsSL https://raw.githubusercontent.com/yedsn/docker-mirror/master/install.sh | sudo bash"
echo -e "  国内环境: curl -fsSL https://gitee.com/hongxiaojian/docker-mirror/raw/master/install.sh | sudo bash -s cn"
echo ""
echo -e "${GREEN}感谢使用 Docker镜像转存工具！${NC}"