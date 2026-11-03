## Alisten

一个音乐播放和房间管理系统，支持从网易云音乐、QQ音乐等平台获取音乐。

## 仓库结构

本仓库是 Go 后端 + React 前端的 monorepo：

| 路径 | 内容 |
| --- | --- |
| `cmd/alisten` | 主服务：房间、队列、播放同步、HTTP 与 WebSocket 接口 |
| `cmd/musichub` | 音乐平台服务：曲库查询、播放链接、响度分析 |
| `frontend/` | 前端：React 18 + TypeScript + Vite + TailwindCSS，支持 PWA |
| `protocol/` | WebSocket v1 协议 IDL 与生成代码 |
| `internal/` | 后端共享包：RPC、鉴权、日志、Nacos、同步工具等 |
| `docs/` | [协议说明](docs/protocol.md) |

`frontend/` 由原 [alisten-ui](https://github.com/bihua-university/alisten-ui) 仓库整体导入，以普通目录保存，不使用 git submodule，克隆后即可直接修改和提交前端代码。前端独立构建为静态站点，Go 服务不托管页面。

## Config

主服务 `alisten` 通过 [RomiChan/protobuf 的 RPC 框架](https://github.com/RomiChan/protobuf/tree/main/rpc) 调用独立的 `musichub`，RPC 地址通过 Nacos 服务发现获取。平台客户端、歌曲缓存与响度分析由 musichub 进程持有；房间、队列和听歌历史由主服务持有。

两个 Go 服务启动时通过 `nacos.Init(serviceName)` 初始化共享客户端，`internal/naconfig` 在首次 `Get()` 时懒加载 JSON。配置快照有效期为 5 分钟，过期后的下一次 `Get()` 会同步刷新并以原子快照替换，未过期或无请求时不会访问 Nacos；后续刷新失败保留旧值。公共环境变量如下：

```bash
export NACOS_SERVER_ADDR='http://127.0.0.1:8848'
export ALISTEN_CONFIG='alisten.json'
export MUSICHUB_CONFIG='musichub.json'
```

`ALISTEN_CONFIG` 和 `MUSICHUB_CONFIG` 分别指定两个服务的 Nacos Data ID，均使用 `DEFAULT_GROUP`。`PORT` 控制监听端口（alisten 默认 `8080`，musichub 默认 `9090`），musichub 注册实例时也使用该端口；`DEBUG` 使用 Go 布尔值控制调试模式。alisten 的 `ADDR` 设置非调试模式的 HTTPS 域名，`TOKEN` 设置管理接口 Bearer Token。这些运行参数不再写入 Nacos JSON。

配置结构分别由 `cmd/alisten/internal/config` 和 `cmd/musichub/internal/config` 独立维护。alisten JSON 只保留持久化房间，musichub JSON 只保留网易云 Cookie。musichub 地址通过 Nacos 服务发现获取，RPC 客户端使用默认超时；传输错误会触发重发现和连接重建，但不会自动重放请求。可设置 `NACOS_SERVICE_IP` 覆盖注册 IP 的自动检测。请求期读取的 Cookie 随刷新生效。

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
go build -o alisten ./cmd/alisten
go build -o musichub ./cmd/musichub

# 两个终端分别启动
./musichub
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

需要覆盖任意模式时，在 `frontend/` 下新建 `.env.[mode].local`（例如 `.env.development.local`），Vite 中它的优先级最高且不会进入版本库。主服务已为 HTTP 接口开启 CORS，前后端可以分开域名运行。

前端协作规范与界面规范见 [frontend/AGENTS.md](frontend/AGENTS.md)、[frontend/DESIGN.md](frontend/DESIGN.md)。
