# Alisten 部署指南

本文档详细介绍如何从克隆项目到使用 Docker/Podman 运行 Alisten 服务的完整步骤。

## 前置要求

- Git
- Docker 或 Podman
- Python 3 (用于安装 podman-compose，如果使用 Podman)

## 部署步骤

### 1. 克隆项目

```bash
# 使用 HTTPS
git clone https://github.com/bihua-university/alisten.git

# 或使用 SSH
git clone git@github.com:bihua-university/alisten.git

# 进入项目目录
cd alisten
```

### 2. 配置文件准备

#### 2.1 准备主服务配置文件

```bash
cp config.json.example config.json
```

#### 2.2 准备 Musiclet 配置文件

```bash
cp musiclet-config.json.example musiclet-config.json
```

**配置说明：**

- `server_url`: Alisten 主服务器地址
- `token`: 认证令牌
- `qiniu_ak`: 七牛云 Access Key
- `qiniu_sk`: 七牛云 Secret Key

**注意**: 容器默认使用 Asia/Shanghai 时区，可以通过环境变量 `TZ` 进行调整。

### 3. 使用 Docker 部署

#### 3.1 完整服务部署（推荐）

使用主 compose 文件同时部署 Alisten 主服务和 Musiclet 工作服务：

```bash
# 编辑配置文件（如果还没有编辑的话）
# vim config.json
# vim musiclet-config.json

# 启动所有服务
docker compose up -d

# 查看服务状态
docker compose ps

# 查看日志
docker compose logs -f

# 停止所有服务
docker compose down
```

#### 3.2 仅部署主服务

如果只需要部署 Alisten 主服务：

```bash
# 启动主服务
docker compose up -d alisten

# 查看主服务状态
docker compose ps alisten

# 查看主服务日志
docker compose logs -f alisten
```

#### 3.3 仅部署 Musiclet 服务

如果只需要部署 Musiclet 工作服务：

```bash
# 启动 Musiclet 服务
docker compose up -d musiclet

# 查看 Musiclet 服务状态
docker compose ps musiclet

# 查看 Musiclet 日志
docker compose logs -f musiclet
```

#### 3.4 直接使用 Docker

构建和运行 Alisten 主服务：

```bash
# 构建主服务镜像
docker build -f docker/alisten.Dockerfile -t alisten .

# 运行主服务容器
docker run -d \
  --name alisten \
  -v $(pwd)/config.json:/app/config.json:ro \
  -e TZ=Asia/Shanghai \
  -p 8080:8080 \
  --restart unless-stopped \
  alisten
```

构建和运行 Musiclet 服务：

```bash
# 构建 Musiclet 镜像
docker build -f docker/musiclet.Dockerfile -t alisten-musiclet .

# 运行 Musiclet 容器
docker run -d \
  --name alisten-musiclet \
  -v $(pwd)/musiclet-config.json:/app/config.json:ro \
  -e TZ=Asia/Shanghai \
  --restart unless-stopped \
  alisten-musiclet
```

### 4. 使用 Podman 部署

#### 4.1 安装 podman-compose

```bash
pip3 install podman-compose
```

#### 4.2 完整服务部署（推荐）

使用主 compose 文件同时部署 Alisten 主服务和 Musiclet 工作服务：

```bash
# 编辑配置文件（如果还没有编辑的话）
# vim config.json
# vim musiclet-config.json

# 启动所有服务
podman-compose up -d

# 查看服务状态
podman-compose ps

# 查看日志
podman-compose logs -f

# 停止所有服务
podman-compose down
```

#### 4.3 仅部署主服务

如果只需要部署 Alisten 主服务：

```bash
# 启动主服务
podman-compose up -d alisten

# 查看主服务状态
podman-compose ps alisten

# 查看主服务日志
podman-compose logs -f alisten
```

#### 4.4 仅部署 Musiclet 服务

如果只需要部署 Musiclet 工作服务：

```bash
# 启动 Musiclet 服务
podman-compose up -d musiclet

# 查看 Musiclet 服务状态
podman-compose ps musiclet

# 查看 Musiclet 日志
podman-compose logs -f musiclet
```

#### 4.5 直接使用 Podman

构建和运行 Alisten 主服务：

```bash
# 构建主服务镜像
podman build -f docker/alisten.Dockerfile -t alisten .

# 运行主服务容器
podman run -d \
  --name alisten \
  -v $(pwd)/config.json:/app/config.json:ro \
  -e TZ=Asia/Shanghai \
  -p 8080:8080 \
  --restart unless-stopped \
  alisten
```

构建和运行 Musiclet 服务：

```bash
# 构建 Musiclet 镜像
podman build -f docker/musiclet.Dockerfile -t alisten-musiclet .

# 运行 Musiclet 容器
podman run -d \
  --name alisten-musiclet \
  -v $(pwd)/musiclet-config.json:/app/config.json:ro \
  -e TZ=Asia/Shanghai \
  --restart unless-stopped \
  alisten-musiclet
```

## 常用管理命令

### 查看日志

```bash
# Docker - 查看所有服务日志
docker compose logs -f

# Docker - 查看特定服务日志
docker logs alisten
docker logs alisten-musiclet

# Podman - 查看所有服务日志
podman-compose logs -f

# Podman - 查看特定服务日志
podman logs alisten
podman logs alisten-musiclet
```

### 重启服务

```bash
# Docker - 重启所有服务
docker compose restart

# Docker - 重启特定服务
docker restart alisten
docker restart alisten-musiclet

# Podman - 重启所有服务
podman-compose restart

# Podman - 重启特定服务
podman restart alisten
podman restart alisten-musiclet
```

### 停止和删除

```bash
# Docker - 停止所有服务
docker compose down

# Docker - 停止和删除特定容器
docker stop alisten && docker rm alisten
docker stop alisten-musiclet && docker rm alisten-musiclet

# Podman - 停止所有服务
podman-compose down

# Podman - 停止和删除特定容器
podman stop alisten && podman rm alisten
podman stop alisten-musiclet && podman rm alisten-musiclet
```

### 重新构建镜像

```bash
# Docker - 重新构建所有服务
docker compose up -d --build

# Docker - 重新构建特定服务
docker build -f docker/alisten.Dockerfile -t alisten . --no-cache
docker build -f docker/musiclet.Dockerfile -t alisten-musiclet . --no-cache

# Podman - 重新构建所有服务
podman-compose up -d --build

# Podman - 重新构建特定服务
podman build -f docker/alisten.Dockerfile -t alisten . --no-cache
podman build -f docker/musiclet.Dockerfile -t alisten-musiclet . --no-cache
```

## 服务说明

### Alisten 主服务

Alisten 主服务提供 Web API 接口，处理音乐搜索、播放等核心功能。启动后会监听 8080 端口。

### Musiclet 工作服务

Musiclet 是一个后台工作服务，负责处理音乐下载、转换等耗时任务。启动后会输出详细的日志信息：

```log
2025/07/08 22:42:15 === Musiclet 启动 ===
2025/07/08 22:42:15 正在读取配置文件...
2025/07/08 22:42:15 尝试读取配置文件: config.json
2025/07/08 22:42:15 配置文件解析成功: ServerURL=https://example.com, Token长度=6
2025/07/08 22:42:15 配置文件读取成功，服务器地址: https://example.com
2025/07/08 22:42:15 Bilibili 配置初始化完成
2025/07/08 22:42:15 任务客户端创建完成
2025/07/08 22:42:15 开始任务循环...
2025/07/08 22:42:15 正在获取任务... (已处理任务数: 0)
2025/07/08 22:42:45 暂无任务，继续等待...
```

## 故障排除

### 1. 配置文件问题

如果看到配置文件相关错误：

- 检查 `config.json` 和 `musiclet-config.json` 文件是否存在
- 验证 JSON 格式是否正确
- 确认文件权限是否正确

### 2. 网络连接问题

如果无法连接到服务器：

- 检查 `server_url` 配置是否正确
- 验证网络连接
- 确认防火墙设置

### 3. 容器启动失败

```bash
# 查看详细错误信息
docker logs alisten
docker logs alisten-musiclet
# 或
podman logs alisten
podman logs alisten-musiclet

# 检查容器状态
docker ps -a
# 或
podman ps -a
```

### 4. 端口冲突

如果主服务无法启动，可能是端口被占用：

```bash
# 检查端口占用情况
netstat -tlnp | grep 8080
# 或
ss -tlnp | grep 8080

# 修改端口映射
docker run -p 8081:8080 alisten
```

### 5. 权限问题

如果遇到权限问题：

```bash
# 检查配置文件权限
ls -la config.json musiclet-config.json

# 修复权限（如果需要）
chmod 644 config.json musiclet-config.json
```

## 生产环境建议

1. **资源限制**: 在生产环境中建议设置适当的 CPU 和内存限制
2. **日志管理**: 配置日志轮转以避免日志文件过大
3. **监控**: 设置健康检查和监控告警
4. **备份**: 定期备份配置文件和重要数据
5. **安全**: 使用非 root 用户运行容器（已在 Dockerfile 中配置）
6. **健康检查**: 容器内置健康检查，定期检查服务状态
7. **负载均衡**: 在高并发场景下，可以部署多个实例并使用负载均衡

### 健康检查

Docker 镜像包含内置的健康检查功能：

```bash
# 查看容器健康状态
docker ps

# 查看详细健康检查信息
docker inspect alisten | grep -A 10 '"Health"'
docker inspect alisten-musiclet | grep -A 10 '"Health"'

# 使用 Docker Compose 查看健康状态
docker compose ps
```

### 资源限制配置

在生产环境中，建议在 `compose.yml` 中添加资源限制：

```yaml
services:
  alisten:
    # ...其他配置...
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M

  musiclet:
    # ...其他配置...
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 256M
        reservations:
          cpus: '0.25'
          memory: 128M
```

## 扩展部署

### 扩展 Musiclet 实例

如果需要运行多个 Musiclet 实例来处理更多任务：

```bash
# 使用 Docker Compose 扩展
docker compose up -d --scale musiclet=3

# 或手动运行多个容器
for i in {1..3}; do
  docker run -d \
    --name alisten-musiclet-$i \
    -v $(pwd)/musiclet-config.json:/app/config.json:ro \
    -e TZ=Asia/Shanghai \
    --network alisten_alisten-network \
    alisten-musiclet
done
```

### 负载均衡配置

对于主服务，可以使用 Nginx 等反向代理进行负载均衡：

```nginx
upstream alisten_backend {
    server localhost:8080;
    server localhost:8081;
    server localhost:8082;
}

server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://alisten_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## 更新部署

当代码更新后：

```bash
# 1. 拉取最新代码
git pull origin main

# 2. 停止现有服务
docker compose down
# 或 podman-compose down

# 3. 重新构建并启动
docker compose up -d --build
# 或 podman-compose up -d --build
```

## 备份和恢复

### 备份配置

```bash
# 创建备份目录
mkdir -p backups/$(date +%Y%m%d)

# 备份配置文件
cp config.json backups/$(date +%Y%m%d)/
cp musiclet-config.json backups/$(date +%Y%m%d)/

# 备份数据库（如果使用本地数据库）
# 具体命令取决于使用的数据库类型
```

### 恢复配置

```bash
# 从备份恢复配置文件
cp backups/20250711/config.json .
cp backups/20250711/musiclet-config.json .

# 重启服务
docker compose restart
```
