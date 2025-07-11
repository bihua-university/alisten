# Musiclet 部署指南

本文档详细介绍如何从克隆项目到使用 Docker/Podman 运行 Musiclet 服务的完整步骤。

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

创建 Musiclet 配置文件：

```bash
cp musiclet-config.json.example musiclet-config.json
```

如果没有示例文件，手动创建 `musiclet-config.json`：

```json
{
    "server_url": "https://your-server-url",
    "token": "your-auth-token-here",
    "qiniu_ak": "your-qiniu-access-key",
    "qiniu_sk": "your-qiniu-secret-key"
}
```

**配置说明：**
- `server_url`: Alisten 主服务器地址
- `token`: 认证令牌
- `qiniu_ak`: 七牛云 Access Key
- `qiniu_sk`: 七牛云 Secret Key

### 3. 使用 Docker 部署

#### 3.1 直接使用 Docker

```bash
# 构建镜像
docker build -f cmd/musiclet/Dockerfile -t alisten-musiclet .

# 运行容器
docker run -d \
  --name alisten-musiclet \
  -v $(pwd)/musiclet-config.json:/app/config.json:ro \
  --restart unless-stopped \
  alisten-musiclet
```

#### 3.2 使用 Docker Compose

```bash
# 启动服务
docker compose -f musiclet-compose.yml up -d

# 查看服务状态
docker compose -f musiclet-compose.yml ps

# 查看日志
docker compose -f musiclet-compose.yml logs -f

# 停止服务
docker compose -f musiclet-compose.yml down
```

### 4. 使用 Podman 部署

#### 4.1 安装 podman-compose

```bash
pip3 install podman-compose
```

#### 4.2 直接使用 Podman

```bash
# 构建镜像
podman build -f cmd/musiclet/Dockerfile -t alisten-musiclet .

# 运行容器
podman run -d \
  --name alisten-musiclet \
  -v $(pwd)/musiclet-config.json:/app/config.json:ro \
  --restart unless-stopped \
  alisten-musiclet
```

#### 4.3 使用 Podman Compose

```bash
# 启动服务
podman-compose -f musiclet-compose.yml up -d

# 查看服务状态
podman-compose -f musiclet-compose.yml ps

# 查看日志
podman-compose -f musiclet-compose.yml logs -f

# 停止服务
podman-compose -f musiclet-compose.yml down
```

## 常用管理命令

### 查看日志

```bash
# Docker
docker logs alisten-musiclet
docker compose -f musiclet-compose.yml logs -f

# Podman
podman logs alisten-musiclet
podman-compose -f musiclet-compose.yml logs -f
```

### 重启服务

```bash
# Docker
docker restart alisten-musiclet
docker compose -f musiclet-compose.yml restart

# Podman
podman restart alisten-musiclet
podman-compose -f musiclet-compose.yml restart
```

### 停止和删除

```bash
# Docker
docker stop alisten-musiclet && docker rm alisten-musiclet
docker compose -f musiclet-compose.yml down

# Podman
podman stop alisten-musiclet && podman rm alisten-musiclet
podman-compose -f musiclet-compose.yml down
```

### 重新构建镜像

```bash
# Docker
docker build -f cmd/musiclet/Dockerfile -t alisten-musiclet . --no-cache
docker compose -f musiclet-compose.yml up -d --build

# Podman
podman build -f cmd/musiclet/Dockerfile -t alisten-musiclet . --no-cache
podman-compose -f musiclet-compose.yml up -d --build
```

## 日志说明

Musiclet 启动后会输出详细的日志信息：

```
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
- 检查 `musiclet-config.json` 文件是否存在
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
docker logs alisten-musiclet
# 或
podman logs alisten-musiclet

# 检查容器状态
docker ps -a
# 或
podman ps -a
```

### 4. 权限问题

如果遇到权限问题：
```bash
# 检查配置文件权限
ls -la musiclet-config.json

# 修复权限（如果需要）
chmod 644 musiclet-config.json
```

## 生产环境建议

1. **资源限制**: 在生产环境中建议设置适当的 CPU 和内存限制
2. **日志管理**: 配置日志轮转以避免日志文件过大
3. **监控**: 设置健康检查和监控告警
4. **备份**: 定期备份配置文件和重要数据
5. **安全**: 使用非 root 用户运行容器（已在 Dockerfile 中配置）

## 扩展部署

如果需要运行多个 Musiclet 实例：

```bash
# 使用 Docker Compose 扩展
docker compose -f musiclet-compose.yml up -d --scale musiclet=3

# 或手动运行多个容器
for i in {1..3}; do
  docker run -d \
    --name alisten-musiclet-$i \
    -v $(pwd)/musiclet-config.json:/app/config.json:ro \
    alisten-musiclet
done
```

## 更新部署

当代码更新后：

```bash
# 1. 拉取最新代码
git pull origin main

# 2. 停止现有服务
docker compose -f musiclet-compose.yml down
# 或 podman-compose -f musiclet-compose.yml down

# 3. 重新构建并启动
docker compose -f musiclet-compose.yml up -d --build
# 或 podman-compose -f musiclet-compose.yml up -d --build
```
