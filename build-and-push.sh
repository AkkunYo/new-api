#!/bin/bash

# Docker 镜像构建和推送脚本
# 用途：自动构建 new-api 镜像并推送到阿里云镜像仓库

set -e  # 遇到错误立即退出

# ==================== 配置区域 ====================
# 阿里云镜像仓库配置
REGISTRY="registry.cn-hangzhou.aliyuncs.com"
NAMESPACE="zkyml"
IMAGE_NAME="newapi-kiro"
VERSION="latest"

# 本地镜像名称
LOCAL_IMAGE="newapi-kiro"

# 清理选项
DEEP_CLEAN=false  # 是否执行深度清理（包括构建缓存）

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ==================== 函数定义 ====================

# 打印信息
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查 Docker 是否运行
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        log_error "Docker 未运行，请先启动 Docker"
        exit 1
    fi
    log_info "Docker 运行正常"
}

# 构建镜像
build_image() {
    log_info "开始构建 Docker 镜像..."
    log_info "镜像名称: ${LOCAL_IMAGE}:${VERSION}"

    # 构建参数：
    # --platform: 目标平台（当前架构）
    # --no-cache: 不使用缓存（确保最新代码）
    # --load: 将镜像加载到本地 Docker
    # -t: 指定镜像标签
    # 注意：多平台构建需要使用 --push 而不是 --load
    if docker buildx build \
        --platform linux/arm64 \
        --no-cache \
        --load \
        -t ${LOCAL_IMAGE}:${VERSION} \
        .; then
        log_info "镜像构建成功"
    else
        log_error "镜像构建失败"
        exit 1
    fi
}

# 打标签
tag_image() {
    local remote_tag="${REGISTRY}/${NAMESPACE}/${IMAGE_NAME}:${VERSION}"
    log_info "给镜像打标签: ${remote_tag}"

    if docker tag ${LOCAL_IMAGE}:${VERSION} ${remote_tag}; then
        log_info "打标签成功"
    else
        log_error "打标签失败"
        exit 1
    fi
}

# 推送镜像
push_image() {
    local remote_tag="${REGISTRY}/${NAMESPACE}/${IMAGE_NAME}:${VERSION}"
    log_info "推送镜像到阿里云: ${remote_tag}"

    if docker push ${remote_tag}; then
        log_info "镜像推送成功"
    else
        log_error "镜像推送失败"
        log_warn "请确保已登录: docker login ${REGISTRY}"
        exit 1
    fi
}

# 清理悬空镜像和容器
cleanup_docker() {
    log_info "清理 Docker 垃圾..."

    # 清理悬空镜像（<none> 标签）
    local dangling_images=$(docker images -f "dangling=true" -q)
    if [ -n "$dangling_images" ]; then
        log_info "清理 $(echo $dangling_images | wc -w) 个悬空镜像..."
        docker rmi $dangling_images 2>/dev/null || log_warn "部分悬空镜像清理失败（可能正在使用）"
    else
        log_info "没有悬空镜像需要清理"
    fi

    # 清理已停止的容器
    local stopped_containers=$(docker ps -a -q -f status=exited)
    if [ -n "$stopped_containers" ]; then
        log_info "清理 $(echo $stopped_containers | wc -w) 个已停止的容器..."
        docker rm $stopped_containers 2>/dev/null || log_warn "部分容器清理失败"
    else
        log_info "没有已停止的容器需要清理"
    fi

    # 深度清理：清理构建缓存
    if [ "$DEEP_CLEAN" = true ]; then
        log_warn "执行深度清理：清理构建缓存..."
        docker builder prune -f
    fi

    log_info "Docker 清理完成"
}

# 显示 Docker 磁盘使用情况
show_docker_usage() {
    log_info "Docker 磁盘使用情况:"
    docker system df
}

# 显示镜像信息
show_image_info() {
    log_info "镜像信息:"
    docker images | grep -E "REPOSITORY|${LOCAL_IMAGE}|${NAMESPACE}/${IMAGE_NAME}" || true
    echo ""
    log_info "拉取命令:"
    echo "  docker pull ${REGISTRY}/${NAMESPACE}/${IMAGE_NAME}:${VERSION}"
}

# ==================== 主流程 ====================

main() {
    echo "========================================"
    echo "  Docker 镜像构建和推送脚本"
    echo "========================================"
    echo ""

    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --deep-clean)
                DEEP_CLEAN=true
                log_warn "启用深度清理模式（将清理构建缓存）"
                shift
                ;;
            *)
                log_error "未知参数: $1"
                echo "用法: $0 [--deep-clean]"
                exit 1
                ;;
        esac
    done

    # 检查 Docker
    check_docker

    # 显示构建前磁盘使用情况
    echo ""
    show_docker_usage
    echo ""

    # 构建镜像
    build_image

    # 打标签
    tag_image

    # 推送镜像
    push_image

    # 显示构建后磁盘使用情况
    echo ""
    show_docker_usage

    # 显示信息
    echo ""
    echo "========================================"
    show_image_info
    echo "========================================"
    echo ""
    log_info "所有操作完成！"
    log_info "提示: 使用 --deep-clean 参数可执行深度清理（清理构建缓存）"
}

# 执行主流程
main "$@"
