# ==================== 第一阶段：构建阶段 ====================
# 什么是基础镜像？
# 基础镜像就像是一个"预装环境"的操作系统模板
# golang:1.22-alpine 表示：使用Go 1.22版本的Alpine Linux系统
# Alpine Linux是一个轻量级的Linux系统，体积小，安全性高

# 为什么需要两个阶段？
# 第一阶段：编译代码，生成可执行文件
# 第二阶段：只保留可执行文件，丢弃编译工具
# 这样可以大大减小最终镜像的体积

FROM golang:1.26-alpine AS builder

# 设置工作目录
# 作用：在容器内创建一个专门的目录来存放项目文件
# 就像在电脑上创建一个项目文件夹
WORKDIR /app

# 安装构建依赖
# git: 用于下载Go模块
# gcc和musl-dev: 某些Go包可能需要C语言编译器
# --no-cache: 不缓存安装包，减小镜像体积
RUN apk add --no-cache git gcc musl-dev

# 复制依赖配置文件
# 为什么要先复制go.mod和go.sum？
# 这是Docker的优化技巧：利用缓存层
# 如果依赖没变，Docker会使用缓存，不用重新下载依赖
COPY go.mod go.sum ./

# 下载项目依赖
# go mod download: 下载所有依赖包
# 这一步会生成一个缓存层，加速后续构建
RUN go mod download

# 复制所有源代码
# 把项目的所有文件复制到容器内
COPY . .

# 构建可执行文件
# CGO_ENABLED=0: 禁用CGO，生成纯静态链接的可执行文件
# GOOS=linux: 指定目标操作系统为Linux
# -ldflags="-s -w": 去除调试信息，减小可执行文件体积
# -o server: 输出文件名为server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

# ==================== 第二阶段：运行阶段 ====================
# 为什么使用alpine:latest？
# 1. 体积小：只有5MB左右
# 2. 安全性高：攻击面小
# 3. 包含基本运行环境：足够运行我们的程序
FROM alpine:latest

# 安装运行时依赖
# ca-certificates: HTTPS证书，用于安全连接
# tzdata: 时区数据，让程序显示正确的时间
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
# TZ=Asia/Shanghai: 使用上海时区（北京时间）
ENV TZ=Asia/Shanghai

# 创建非root用户
# 为什么要创建新用户？
# 安全最佳实践：不使用root用户运行应用
# 如果程序被攻击，攻击者只能获得普通用户权限，降低风险
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制可执行文件
# --from=builder: 从第一阶段（builder）复制文件
# --chown=appuser:appgroup: 设置文件所有者为新创建的用户
# 只复制编译好的server文件，不复制源代码
COPY --from=builder --chown=appuser:appgroup /app/server .

# 创建上传目录并设置权限
# mkdir -p: 创建多级目录
# chown -R: 递归设置目录所有者
# 这样appuser用户就有权限在uploads目录下创建文件
RUN mkdir -p uploads/avatars uploads/videos && \
    chown -R appuser:appgroup uploads

# 切换到非root用户
# 之后的所有操作都以appuser身份运行
USER appuser

# 暴露端口
# EXPOSE: 声明容器运行时监听的端口
# 这只是声明，不会真正打开端口
# 真正的端口映射在docker run时指定
EXPOSE 8888

# 健康检查
# Docker会定期执行这个命令检查容器是否健康
# --interval=30s: 每30秒检查一次
# --timeout=3s: 超时时间为3秒
# --start-period=5s: 容器启动后5秒才开始检查
# --retries=3: 连续失败3次才认为不健康
# wget --spider: 只检查连接，不下载内容
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8888/ping || exit 1

# 启动服务
# CMD: 容器启动时执行的命令
# 只有一个命令，使用数组形式
CMD ["./server"]
