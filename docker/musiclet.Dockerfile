# 使用官方 Go 镜像作为构建环境
FROM golang:1.24-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的系统依赖
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go.mod 和 go.sum 文件并下载依赖（利用 Docker 缓存）
COPY go.mod go.sum ./
RUN go mod download

# 复制整个项目源代码
COPY . .

# 构建 musiclet 应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -o musiclet ./cmd/musiclet

# 使用轻量级的 alpine 镜像作为运行环境
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN addgroup -g 1001 -S musiclet && \
    adduser -u 1001 -S musiclet -G musiclet

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/musiclet .

# 更改文件所有者
RUN chown -R musiclet:musiclet /app

# 切换到非 root 用户
USER musiclet

# 设置健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep musiclet || exit 1

# 运行应用
CMD ["./musiclet"]
