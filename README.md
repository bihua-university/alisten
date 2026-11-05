## Alisten

一个音乐播放和房间管理系统，支持从网易云音乐、QQ音乐等平台获取音乐。

## 仓库结构

本仓库是 Rust gateway/livehouse-worker + Go livehouse-api/musichub + React 前端的 monorepo：

| 路径 | 内容 |
| --- | --- |
| `gateway/` | Pingora 接入网关：Nacos 发现、HTTP 代理、基于 Redis 归属的 WebSocket 转发 |
| `livehouse-worker/` | Tower/Hyper Rust 房间节点：队列、成员、聊天、播放同步、Redis 持久化与租约 fencing |
| `crates/nacos-client/` | Rust 服务共用的轻量 Nacos client |
| `cmd/livehouse-api` | Go HTTP 服务：房间列表、点歌准备、搜索与推荐，不持有房间状态 |
| `cmd/musichub` | 音乐平台服务：曲库查询、播放链接、响度分析 |
| `frontend/` | React 18 + TypeScript + Vite + TailwindCSS，支持 PWA |
| `internal/` | Go 共享包：`rpc/livehouse` worker client、鉴权与 Nacos 等；HTTP handler 位于 `cmd/livehouse-api/internal/service` |
| `docs/` | 设计与协议说明 |

前端独立构建为静态站点，Go 服务不托管页面。

## Architecture

```text
浏览器 ── HTTP/WS ─▶ gateway :8080/:443
                       │
              /api/*   │     /ws（按 roomId 查询 Redis 归属）
                       ▼                         ▼
               livehouse-api :8081 ◀── /rpc ── livehouse-worker :8082
                       │                              │  心跳、归属、房间锚点
                       │                              ▼
                       │                           Redis
                       └──────────────▶ musichub :9090
```

livehouse-worker 是唯一房间状态持有者。节点只在状态变更时持久化播放锚点，使用租约和 revision 条件写入防止双主与迟到写；节点恢复后从锚点推导进度并跳过已结束歌曲。gateway 通过 Redis 节点心跳和 `room -> node` 归属表转发，不保存房间状态，并直接拒绝外部 `/rpc` 与 `/internal/*`。

livehouse-api 提供外部 `/api` HTTP 接口。房间相关请求统一转换为 worker 的内部 `POST /rpc`；音乐搜索、歌单、推荐以及点歌的播放地址和响度准备都在 Go 侧完成。worker 仅提供 `/ws`、内部 `/rpc` 和健康检查，不依赖音乐提供方。

## Config

livehouse-api、musichub 与 gateway 使用 `NACOS_SERVER_ADDR`，格式为 `http(s)://[user:password@]host:8848[/nacos]?namespace=...`。worker 的节点发现与房间归属只使用 Redis。

```bash
export NACOS_SERVER_ADDR='http://127.0.0.1:8848'
export LIVEHOUSE_API_CONFIG='livehouse-api.json'
export MUSICHUB_CONFIG='musichub.json'
export REDIS_URL='redis://127.0.0.1:6379'
```

主要环境变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | gateway `8080`、livehouse-api `8081`、musichub `9090` | 各服务监听端口 |
| `LIVEHOUSE_ADDR` | `127.0.0.1:8082` | worker 监听地址，同时写入 Redis 心跳供 gateway 和 RPC 连接 |
| `LIVEHOUSE_API` | `http://127.0.0.1:8082` | livehouse-api 的 worker RPC 入口；实际节点由 Redis 归属表决定并自动转发 |
| `REDIS_URL` | `redis://127.0.0.1:6379` | gateway 与 worker 共用的 Redis |
| `LIVEHOUSE_REDIS_PREFIX` | `livehouse` | Redis key 前缀，环境间需隔离 |
| `LIVEHOUSE_MAX_MEMBERS` | `100` | 单房间成员上限 |
| `LIVEHOUSE_NODE_ID` | 随机 UUID | worker 节点 ID |
| `INTERNAL_API_TOKEN` | 空 | 内部 `/rpc` Bearer Token，生产环境应设置 |
| `LIVEHOUSE_API_NAME` | `livehouse-api` | gateway 的 Nacos 服务名 |
| `CANARY_NACOS_NAMESPACE` | 空 | Canary namespace；同时需使用隔离的 Redis 前缀 |
| `RUST_LOG` / `DEBUG` | — | Rust / Go 日志配置 |

`LIVEHOUSE_API_CONFIG` 和 `MUSICHUB_CONFIG` 是 Nacos Data ID。livehouse-api 启动时将配置中的持久化房间通过 `room.ensure` 注册到 Redis，不覆盖已有队列。

## API 接口

外部接口统一使用 `/api` 前缀，WebSocket `/ws` 使用 [protocol v2](docs/protocol.md)。只保留以下 HTTP 接口：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `POST` | `/api/house/add` | 创建房间 |
| `GET` / `POST` | `/api/house/search` | 搜索房间 |
| `POST` | `/api/music/pick` | 准备并入队歌曲 |
| `POST` | `/api/music/search` | 搜索歌曲或读取歌单歌曲 |
| `POST` | `/api/music/searchsonglist` | 搜索歌单 |
| `POST` | `/api/music/recommend` | 获取推荐 |
| `POST` | `/api/music/resolve` | 获取最新播放 URL；同 key 请求合并并缓存 5 秒 |

搜索、推荐和点歌不需要建立 WebSocket。旧的播放、队列、点赞、投票、设置和成员 HTTP 适配已删除；这些实时操作只通过 `/ws` 或内部 RPC 完成。

### 点歌

```json
{
  "houseId": "房间ID",
  "password": "房间密码",
  "id": "音乐ID（可选）",
  "name": "音乐名称",
  "source": "netease"
}
```

`source` 支持 `netease`、`qq`、`kuwo`；`id` 与 `name` 至少提供一个。成功返回 `{ "name", "artist", "id", "source" }`，错误返回 `{ "error" }`。搜索与推荐返回 `{ "items", "totalSize" }`，字段与协议 DTO 一致。

## Build and run

```bash
go build -o livehouse-api ./cmd/livehouse-api
go build -o musichub ./cmd/musichub
cargo build --release --locked -p gateway -p livehouse-worker
```

启动前先运行 Nacos 和 Redis，然后分别启动：

```bash
./musichub
./target/release/livehouse-worker
LIVEHOUSE_API_CONFIG=livehouse-api.json ./livehouse-api
./target/release/gateway
```

Windows 下二进制带 `.exe`。worker 在 Windows 使用系统 TLS，其他平台使用 rustls；gateway 始终使用 rustls。`build.sh` 构建所有服务，Rust 产物位于 `target/release`。`.deploy` 部署配置不在本次改造范围内。

### HTTPS 与证书目录

gateway 读取 acme.sh 输出的 `fullchain.pem` 与 `privkey.pem`，在 `TLS_PORT`（默认 `443`）终结 HTTPS，并每 30 秒检查证书热加载。也可通过 `TLS_CERT_FILE`、`TLS_KEY_FILE` 指定证书。启用后前端使用 `https://music.example.com` 和 `wss://music.example.com`。

## 前端

```bash
cd frontend
npm ci
npm run dev
npm run type-check
npm run lint
npm run build
```

构建时通过 `VITE_WS_URL`、`VITE_API_URL`、`VITE_APP_NAME` 注入地址，默认分别为 `ws://localhost:8080`、`http://localhost:8080` 和音乐共享房间。前端规范见 [frontend/AGENTS.md](frontend/AGENTS.md) 与 [frontend/DESIGN.md](frontend/DESIGN.md)。
