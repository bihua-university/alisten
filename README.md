## Alisten

一个音乐播放和房间管理系统，支持从网易云音乐、QQ音乐等平台获取音乐。

## 仓库结构

本仓库是 Rust gateway + Go 后端 + React 前端的 monorepo：

| 路径 | 内容 |
| --- | --- |
| `gateway/` | Pingora 0.9.1 接入网关：Nacos 发现、REST 与 WebSocket 透明代理 |
| `cmd/room-service` | 房间节点：房间、队列、播放同步、HTTP 与 WebSocket 业务处理 |
| `cmd/musichub` | 音乐平台服务：曲库查询、播放链接、响度分析 |
| `frontend/` | 前端：React 18 + TypeScript + Vite + TailwindCSS，支持 PWA |
| `protocol/` | WebSocket v1 协议 IDL 与生成代码 |
| `internal/` | Go 后端共享包：RPC、鉴权、日志、Nacos、同步工具等 |
| `docker/` | Linux/amd64 镜像构建配置 |
| `docs/` | [协议说明](docs/protocol.md) |

`frontend/` 由原 [alisten-ui](https://github.com/bihua-university/alisten-ui) 仓库整体导入，以普通目录保存，不使用 git submodule，克隆后即可直接修改和提交前端代码。前端独立构建为静态站点，Go 服务不托管页面。

## Architecture

```text
浏览器 / REST 客户端 ── HTTP/HTTPS ─▶ gateway :8080/:443
                                           │ Nacos 发现 room-service
                                           ▼
                                     room-service :8081
                                           │ Nacos 发现 musichub
                                           ▼
                                     musichub :9090
```

gateway 是当前唯一业务接入点，现有 `/house/*`、`/music/*`、`/config/*` 和 `/server` 均保持原路径。当前仅选择一个健康的 room-service 实例，发现结果缓存 5 秒，连接失败会使缓存失效；尚未实现多节点负载均衡或按房间分片。room-service 与 musichub 启动后通过 Nacos 注册、5 秒心跳保持实例，退出时注销。

## Config

三个服务共用 `NACOS_SERVER_ADDR`，支持 `http(s)://[user:password@]host:8848[/nacos]?namespace=...`。room-service 和 musichub 使用 `internal/nacos` 与 `internal/naconfig`；gateway 使用等价的轻量 Rust Nacos client，只实现认证和健康实例发现。

```bash
export NACOS_SERVER_ADDR='http://127.0.0.1:8848'
export ROOM_SERVICE_CONFIG='room-service.json'
export MUSICHUB_CONFIG='musichub.json'
```

`ROOM_SERVICE_CONFIG` 和 `MUSICHUB_CONFIG` 分别指定两个 Go 服务的 Nacos Data ID，均使用 `DEFAULT_GROUP`。`PORT` 控制监听端口（gateway 默认 `8080`，room-service 默认 `8081`，musichub 默认 `9090`）；`ROOM_SERVICE_NAME` 指定 gateway 发现的服务名，默认 `room-service`；`RUST_LOG` 控制 gateway 日志；`DEBUG` 控制 Go 服务调试日志；room-service 的 `TOKEN` 设置管理接口 Bearer Token。可设置 `NACOS_SERVICE_IP` 覆盖 Go 实例注册 IP 的自动检测。

配置结构分别由 `cmd/room-service/internal/config` 和 `cmd/musichub/internal/config` 独立维护。room-service JSON 只保留持久化房间，musichub JSON 只保留网易云 Cookie。RPC 传输错误会触发重发现和连接重建，但不会自动重放请求。请求期读取的 Cookie 随刷新生效。

共享 RPC IDL 和 golite 生成文件位于 `internal/proto/musichub`，方法名为 `musichub.v1.*`。

## Features

- 🎵 支持多平台音乐源（网易云音乐、QQ音乐）
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
    "source": "音乐源（netease/qq/kuwo）"
}
```

**请求参数说明**:

- `houseId`: 要点歌的房间ID
- `password`: 房间密码
- `id`: 音乐的唯一标识符（可选，如果未提供则会根据name和source搜索）
- `name`: 音乐名称或搜索关键词
- `source`: 音乐平台来源
  - `netease`: 网易云音乐
  - `qq`: QQ音乐
  - `kuwo`: 酷我音乐

**响应示例**:

成功时:

```json
{
    "name": "音乐名称",
    "source": "netease",
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
go build -o room-service ./cmd/room-service
go build -o musichub ./cmd/musichub
cd gateway && cargo build --release --locked && cd ..

# 三个终端按顺序启动
./musichub
ROOM_SERVICE_CONFIG=room-service.json ./room-service
cd gateway && ../gateway/target/release/gateway
```

Linux/amd64 推荐直接使用多阶段 Docker 构建，无需在本机安装交叉编译工具链：

```bash
docker compose build gateway room-service musichub
docker compose up -d gateway room-service musichub
```

### HTTPS 与 acme.sh 证书目录

acme.sh 的签发、续期和部署流程由外部环境管理，gateway 只读取其输出的 `fullchain.pem` 与 `privkey.pem`。Pingora 直接在 `TLS_PORT`（默认 `443`）终结 HTTPS，并每 30 秒检查证书文件；新证书和私钥校验通过后热加载，无需重启，也不经过其他代理。

将 acme.sh 的证书目录通过 `CERT_DIR` 挂载，并启用 HTTPS override：

```bash
export CERT_DIR='/path/to/acme-certs'
docker compose -f compose.yml -f compose.https.yml up -d gateway
```

目录中的文件名固定为 `fullchain.pem` 和 `privkey.pem`，且需要允许容器内 UID/GID `65532` 读取。非容器运行时可设置 `TLS_CERT_FILE`、`TLS_KEY_FILE` 和可选的 `TLS_PORT`。启用后前端使用 `https://music.example.com` 和 `wss://music.example.com`；生产环境可通过防火墙或 Compose override 关闭 `8080` 的公网映射。

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

构建时通过环境变量注入后端地址：`VITE_WS_URL`、`VITE_API_URL`、`VITE_APP_NAME`。没有对应环境变量时前端回退到 `src/utils/config.ts` 中的默认值 `ws://localhost:8080` 与 `http://localhost:8080`，因此本机启动 gateway 即可联调，无需改前端地址。

需要覆盖任意模式时，在 `frontend/` 下新建 `.env.[mode].local`（例如 `.env.development.local`），Vite 中它的优先级最高且不会进入版本库。room-service 的 CORS 响应经 gateway 原样转发，前后端可以分开域名运行。

前端协作规范与界面规范见 [frontend/AGENTS.md](frontend/AGENTS.md)、[frontend/DESIGN.md](frontend/DESIGN.md)。
