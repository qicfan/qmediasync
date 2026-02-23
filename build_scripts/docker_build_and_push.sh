#!/bin/bash

# QMediaSync Docker镜像构建和推送脚本
# 使用docker buildx直接构建amd64和arm64多架构镜像并推送
# 适用于Linux环境

set -e

cd ../
echo "已切换工作目录：$(pwd)"
# 默认参数
DOCKER_HUB_USERNAME="qicfan"
DOCKER_HUB_PASSWORD=""
IMAGE_NAME="qmediasync"
VERSION=""
DOCKERFILE="Dockerfile"

# 检测是否从build_and_release.sh调用
if [ -n "$BUILD_AND_RELEASE_CALL" ]; then
    DOCKERFILE="Dockerfile_local"
    echo "检测到从build_and_release.sh调用，使用Dockerfile_local"
fi

# 显示帮助信息
show_help() {
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -u, --username USERNAME    Docker Hub用户名"
    echo "  -p, --password PASSWORD    Docker Hub密码"
    echo "  -i, --image IMAGE_NAME     镜像名称 (默认: qmediasync)"
    echo "  -v, --version VERSION      版本标签 (默认: 自动检测Git标签)"
    echo "  -f, --dockerfile FILE      Dockerfile文件 (默认: Dockerfile)"
    echo "  -h, --help                 显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 -u myusername -p mypassword -v v1.0.0"
    echo "  $0 --username myusername --password mypassword --image myapp"
    echo "  $0 --dockerfile Dockerfile_local"
}

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -u|--username)
            DOCKER_HUB_USERNAME="$2"
            shift 2
            ;;
        -p|--password)
            DOCKER_HUB_PASSWORD="$2"
            shift 2
            ;;
        -i|--image)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -f|--dockerfile)
            DOCKERFILE="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "错误: 未知选项 $1"
            show_help
            exit 1
            ;;
    esac
done

echo "========================================"
echo "QMediaSync Docker镜像构建和推送脚本"
echo "========================================"

# 检查Docker是否安装
if ! command -v docker &> /dev/null; then
    echo "错误: Docker未安装或不在PATH中"
    exit 1
fi

# 检查是否在Git仓库中
if [ ! -d ".git" ]; then
    echo "错误: 不在Git仓库中"
    exit 1
fi

# 确定版本标签
if [ -n "$VERSION" ]; then
    # 使用提供的版本参数
    TAG="$VERSION"
    echo "使用提供的版本: $TAG"
else
    # 自动检测现有标签
    TAG=$(git describe --tags --exact-match 2>/dev/null || true)
    if [ -z "$TAG" ]; then
        echo "错误: 当前HEAD没有关联的Git标签"
        echo "请创建并推送标签: git tag vX.X.X && git push origin vX.X.X"
        exit 1
    fi
    echo "检测到Git标签: $TAG"
fi

# 获取当前时间作为构建日期
BUILD_DATE=$(date '+%Y-%m-%d %H:%M:%S')

# 检查Docker Hub认证信息并构建完整镜像名称
if [ -z "$DOCKER_HUB_PASSWORD" ]; then
    echo "未提供Docker Hub密码，检查现有登录状态..."
    
    # 检查Docker是否已登录
    if docker info 2>/dev/null | grep -q "Username:"; then
        echo "✓ 检测到Docker已登录，将使用现有登录状态"
        SKIP_PUSH=false
        
        # 从Docker登录信息中获取用户名
        DOCKER_USERNAME=$(docker info 2>/dev/null | grep "Username:" | awk '{print $2}')
        if [ -n "$DOCKER_USERNAME" ]; then
            # 构建完整的镜像名称：用户名/镜像名
            FULL_IMAGE_NAME="$DOCKER_USERNAME/$IMAGE_NAME"
            echo "使用Docker Hub用户名: $DOCKER_USERNAME"
            echo "完整镜像名称: $FULL_IMAGE_NAME"
        else
            echo "错误: 无法从Docker登录信息中获取用户名"
            SKIP_PUSH=true
        fi
    else
        echo "警告: Docker未登录，将跳过镜像推送"
        SKIP_PUSH=true
        FULL_IMAGE_NAME="$IMAGE_NAME"
    fi
else
    SKIP_PUSH=false
    echo "Docker Hub认证信息已提供"
    
    # 使用默认用户名构建完整镜像名称
    FULL_IMAGE_NAME="$DOCKER_HUB_USERNAME/$IMAGE_NAME"
    echo "使用Docker Hub用户名: $DOCKER_HUB_USERNAME"
    echo "完整镜像名称: $FULL_IMAGE_NAME"
fi

# 检查并设置buildx构建器
setup_buildx() {
    echo "设置docker buildx构建器..."
    
    # 检查buildx是否可用
    if ! docker buildx version &> /dev/null; then
        echo "错误: docker buildx不可用，请确保Docker版本支持buildx"
        exit 1
    fi
    
    # 创建并启动buildx构建器（如果不存在）
    if ! docker buildx ls | grep -q "multiarch-builder"; then
        echo "创建多架构构建器..."
        docker buildx create --name multiarch-builder --driver docker-container --use --driver-opt env.http_proxy=http://192.168.0.4:10809 --driver-opt env.https_proxy=http://192.168.0.4:10809
        if [ $? -ne 0 ]; then
            echo "错误: 创建buildx构建器失败"
            exit 1
        fi
    else
        echo "使用现有的多架构构建器"
        docker buildx use multiarch-builder
    fi
    
    # 启动构建器
    docker buildx inspect --bootstrap
    if [ $? -ne 0 ]; then
        echo "错误: 启动buildx构建器失败"
        exit 1
    fi
    
    echo "✓ buildx构建器设置完成"
}

# 使用buildx构建多架构镜像
build_multi_arch_image() {
    local tag="$1"
    local build_date="$2"
    local skip_push="$3"
    
    echo "使用docker buildx构建多架构镜像..."
    echo "支持的架构: linux/amd64, linux/arm64"
    
    # 构建参数
    local build_args=""
    if [ "$skip_push" = "false" ]; then
        build_args="--push"
        echo "模式: 构建并推送"
    else
        build_args="--load"
        echo "模式: 仅构建（本地加载）"
    fi
    
    # 使用buildx构建多架构镜像
    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        --build-arg "VERSION=$tag" \
        --build-arg "BUILD_DATE=$build_date" \
        -f "$DOCKERFILE" \
        -t "$FULL_IMAGE_NAME:$tag" \
        -t "$FULL_IMAGE_NAME:latest" \
        $build_args \
        .
    
    if [ $? -ne 0 ]; then
        echo "错误: 多架构镜像构建失败"
        return 1
    fi
    
    echo "✓ 多架构镜像构建成功"
    return 0
}

# 登录Docker Hub（仅在需要时）
login_to_docker_hub() {
    # 检查是否已登录
    if docker info 2>/dev/null | grep -q "Username:"; then
        echo "✓ Docker已登录，跳过登录步骤"
        return 0
    fi
    
    # 检查是否有认证信息
    if [ -z "$DOCKER_HUB_USERNAME" ] || [ -z "$DOCKER_HUB_PASSWORD" ]; then
        echo "错误: 需要Docker Hub认证信息但未提供"
        return 1
    fi
    
    echo "登录Docker Hub..."
    echo "$DOCKER_HUB_PASSWORD" | docker login -u "$DOCKER_HUB_USERNAME" --password-stdin
    
    if [ $? -ne 0 ]; then
        echo "错误: Docker Hub登录失败"
        return 1
    fi
    
    echo "✓ Docker Hub登录成功"
    return 0
}

# 主执行流程
echo "开始Docker镜像构建流程..."

# 1. 设置buildx构建器
setup_buildx

# 2. 检查推送状态并处理登录
if [ "$SKIP_PUSH" != "true" ]; then
    echo "模式: 构建并推送镜像"
    if ! login_to_docker_hub; then
        echo "错误: Docker Hub登录失败，终止流程"
        exit 1
    fi
else
    echo "模式: 仅构建镜像（不推送）"
fi

# 3. 使用buildx构建多架构镜像
if ! build_multi_arch_image "$TAG" "$BUILD_DATE" "$SKIP_PUSH"; then
    echo "错误: 镜像构建失败，终止流程"
    exit 1
fi

# 4. 显示构建结果
if [ "$SKIP_PUSH" != "true" ]; then
    echo "========================================"
    echo "Docker镜像构建和推送完成!"
    echo "镜像标签: $FULL_IMAGE_NAME:$TAG"
    echo "镜像标签: $FULL_IMAGE_NAME:latest"
    echo "支持的架构: linux/amd64, linux/arm64"
    echo "========================================"
else
    echo "========================================"
    echo "Docker镜像构建完成（跳过推送）"
    echo "镜像标签: $FULL_IMAGE_NAME:$TAG"
    echo "镜像标签: $FULL_IMAGE_NAME:latest"
    echo "支持的架构: linux/amd64, linux/arm64"
    echo "========================================"
fi