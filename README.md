## Alisten

一个音乐播放和房间管理系统，支持从 Bilibili、网易云音乐、QQ音乐等平台获取音乐。

## Config

主服务 `alisten` 通过 [RomiChan/protobuf 的 RPC 框架](https://github.com/RomiChan/protobuf/tree/main/rpc) 调用独立的 `alisten-musicd`。平台客户端、歌曲缓存与响度分析由 musicd 进程持有；房间、队列和听歌历史由主服务持有。

复制 `config.json.example` 与 `musicd-config.json.example` 为对应配置文件。

- 主服务：`music.rpc.address` 指定 RPC 地址，`music.rpc.http_url` 指定 worker HTTP 转发地址，`music.rpc.timeout_seconds` 默认 210 秒。
- musicd：`music.cookie` 保存网易云 Cookie，`token` 验证 Python worker 请求。
- `persist` 定义主服务持久化房间，`token` 验证主服务 Cookie 管理接口。
- 本机使用 `127.0.0.1:9090` 与 `http://127.0.0.1:8081`；Compose 示例使用 `alisten-musicd` 服务名。

共享 RPC IDL 和 golite 生成文件位于 `internal/proto/musicd`，方法名为 `musicd.v1.*`。生成与详细配置说明见 [部署指南](docs/deploy.md)。

## Features

- 🎵 支持多平台音乐源（Bilibili、网易云音乐、QQ音乐）
- 🏠 房间管理系统，支持持久化房间配置
- 🎶 HTTP API 点歌功能，支持通过 REST API 进行点歌

## API 接口

WebSocket `/server` 使用统一的 [protocol v1](docs/protocol.md)，请求、响应、广播及错误均由 `protocol/ws.idl.json` 定义。旧 `{ action, data }` 消息需迁移为带 `version`、`kind`、`type` 和 `requestId` 的请求。HTTP 接口继续使用下述请求与响应结构。

### 点歌接口

**POST** `/music/pick`

通过 HTTP POST 请求为指定房间点歌。

**请求体**:

```json
{
    "houseId": "房间ID",
    "password": "房间密码",
    "id": "音乐ID（可选）",
    "name": "音乐名称",
    "source": "音乐源（wy/qq/db）"
}
```

**请求参数说明**:

- `houseId`: 要点歌的房间ID
- `password`: 房间密码
- `id`: 音乐的唯一标识符（可选，如果未提供则会根据name和source搜索）
- `name`: 音乐名称或搜索关键词
- `source`: 音乐平台来源
  - `wy` 或 `netease`: 网易云音乐
  - `qq`: QQ音乐
  - `db`: Bilibili（支持 BV 号）

**响应示例**:

成功时:

```json
{
    "name": "音乐名称",
    "source": "wy",
    "id": "音乐ID"
}
```

错误时:

```json
{
    "error": "错误信息"
}
```

## Build and run

```bash
go build -o alisten ./cmd/alisten
go build -o alisten-musicd ./cmd/alisten-musicd

# 两个终端分别启动（本机配置需使用 127.0.0.1 地址）
./alisten-musicd
./alisten
```

## 部署指南

详细的部署说明请参考 [部署指南](docs/deploy.md)，包含：

- Docker/Podman 容器化部署
- 完整的配置文件说明
- 生产环境建议
- 故障排除指南

## Docker 快速启动

使用 Docker Compose 一键启动所有服务：

```bash
# 克隆项目
git clone https://github.com/bihua-university/alisten.git
cd alisten

# 准备配置文件
cp config.json.example config.json
cp musicd-config.json.example musicd-config.json

# 启动服务
docker compose up -d --build alisten alisten-musicd
```
