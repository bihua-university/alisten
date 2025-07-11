# 使用官方 Go 镜像作为构建环境
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的系统依赖
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go.mod 和 go.sum 文件并下载依赖（利用 Docker 缓存）
COPY go.mod go.sum ./
RUN go mod download

# 复制整个项目源代码
COPY . .

# 构建 alisten 应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -o alisten .

# 使用轻量级的 alpine 镜像作为运行环境
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN addgroup -g 1001 -S alisten && \
    adduser -u 1001 -S alisten -G alisten

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/alisten .

# 创建配置文件目录
RUN mkdir -p /app/config

# 更改文件所有者
RUN chown -R alisten:alisten /app

# 切换到非 root 用户
USER alisten

# 设置健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep alisten || exit 1

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["./alisten"]
