# 使用官方 Python 镜像 (alpine 版本更安全轻量)
FROM python:3.11-alpine

# 设置工作目录
WORKDIR /app

# 安装系统依赖
# - ffmpeg: yt-dlp 需要用于音频转换
# - ca-certificates: HTTPS 请求所需
# - curl: 健康检查用
# - gcc, musl-dev: 编译 Python 包所需
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    curl \
    gcc \
    musl-dev \
    postgresql-dev \
    && rm -rf /var/cache/apk/*

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户 (Alpine 风格)
RUN addgroup -g 1001 -S musiclet && \
    adduser -u 1001 -S musiclet -G musiclet

# 复制 requirements.txt 并安装 Python 依赖（利用 Docker 缓存）
COPY cmd/musiclet-py/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# 复制 musiclet-py 源代码
COPY cmd/musiclet-py/ ./

# 创建下载目录并设置权限
RUN mkdir -p downloads && \
    chown -R musiclet:musiclet /app

# 切换到非 root 用户
USER musiclet

# 设置 Python 路径
ENV PYTHONPATH=/app

# 暴露端口（如果需要的话，根据实际情况调整）
# EXPOSE 8080

# 设置健康检查
# 检查进程是否运行，可以根据实际情况调整检查方式
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD pgrep -f "python.*main.py" || exit 1

# 运行应用
CMD ["python", "main.py"]