## Alisten

一个音乐播放和房间管理系统，支持从 Bilibili、网易云音乐、QQ音乐等平台获取音乐。

## 仓库结构

本仓库是 Go 后端 + React 前端的 monorepo：

| 路径 | 内容 |
| --- | --- |
| `cmd/alisten` | 主服务：房间、队列、播放同步、HTTP 与 WebSocket 接口 |
| `cmd/alisten-musicd` | 音乐平台服务：曲库查询、播放链接、响度分析 |
| `cmd/musiclet-py` | 可选的通用 URL 下载 worker |
| `frontend/` | 前端：React 18 + TypeScript + Vite + TailwindCSS，支持 PWA |
| `protocol/` | WebSocket v1 协议 IDL 与生成代码 |
| `internal/` | 后端共享包：RPC、鉴权、日志、同步工具等 |
| `compose.yml` | Docker Compose 编排 |
| `docs/` | [部署指南](docs/deploy.md) 与 [协议说明](docs/protocol.md) |

`frontend/` 由原 [alisten-ui](https://github.com/bihua-university/alisten-ui) 仓库整体导入，以普通目录保存，不使用 git submodule，克隆后即可直接修改和提交前端代码。前端独立构建为静态站点，产物 `frontend/dist` 由 CDN 或任意静态服务器托管，Go 服务不托管页面。

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

## 前端

前端位于 `frontend/`，与 Go 服务分开构建：

```bash
cd frontend
npm ci              # 安装依赖
npm run dev         # 开发服务器 http://localhost:3000
npm run type-check  # 类型检查
npm run lint        # ESLint
npm run build       # 生产构建，产物在 frontend/dist
```

构建时通过环境变量注入后端地址：`VITE_WS_URL`、`VITE_API_URL`、`VITE_APP_NAME`。仓库只提交生产配置 `frontend/.env.production`；没有对应环境变量时前端回退到 `src/utils/config.ts` 中的默认值 `ws://localhost:8080` 与 `http://localhost:8080`，因此本机跑主服务（`debug: true`）即可联调。

需要覆盖任意模式时，在 `frontend/` 下新建 `.env.[mode].local`（例如 `.env.development.local`），Vite 中它的优先级最高且不会进入版本库。主服务已为 HTTP 接口开启 CORS，前后端可以分开域名部署。

前端协作规范与界面规范见 [frontend/AGENTS.md](frontend/AGENTS.md)、[frontend/DESIGN.md](frontend/DESIGN.md)。

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
