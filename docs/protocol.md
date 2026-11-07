# Livehouse WebSocket protocol v2

livehouse 是唯一的房间状态持有者。浏览器只通过 gateway 的 `/ws` 建立 WebSocket；Go `livehouse-api` 提供 HTTP API，并通过内部 `POST /rpc` 执行房间操作。音乐搜索、歌单搜索、推荐和点歌准备均属于 HTTP 边界，不经过 WebSocket。

## 连接与信封

连接地址：`ws(s)://<gateway>/ws?roomId=<encoded-id>&password=<encoded-password>`。gateway 只解析房间 ID，根据 Redis 健康节点和房间归属转发帧。`/rpc` 与 `/internal/*` 由 gateway 直接拒绝，不能从外部访问。

只接受不超过 64 KiB 的 UTF-8 JSON 文本帧，协议版本固定为 `2`。请求与响应通过 `requestId` 关联，不同请求可以乱序完成；每条连接最多同时处理 16 个请求。写请求超时或断线后不会自动重放。

```json
{"version":2,"kind":"request","type":"clock.ping","requestId":"r1","data":{"clientTimeMs":123}}
```

成功响应：

```json
{"version":2,"kind":"response","type":"clock.ping","requestId":"r1","data":{"clientTimeMs":123,"serverTimeMs":456}}
```

失败响应保留关联信息，`data` 为 `{}`：

```json
{"version":2,"kind":"response","type":"queue.remove","requestId":"r2","data":{},"error":{"code":"NOT_FOUND","message":"未找到要删除的音乐"}}
```

无法关联请求的错误使用 `protocol.error` 事件。错误码包括 `INVALID_MESSAGE`、`UNSUPPORTED_VERSION`、`UNKNOWN_TYPE`、`INVALID_ARGUMENT`、`NOT_FOUND`、`UNAUTHORIZED`、`RATE_LIMITED`、`CONFLICT`、`PROVIDER_ERROR`、`UNAVAILABLE`、`LEASE_LOST` 和 `INTERNAL_ERROR`。

## WebSocket 房间操作

| type | 请求 data | 成功响应 data |
| --- | --- | --- |
| `clock.ping` | `{ clientTimeMs }` | `{ clientTimeMs, serverTimeMs }` |
| `room.sync` | `{}` | 完整 `RoomSnapshot` |
| `chat.send` | `{ content }` | `{}` |
| `member.update` | `{ name, email, clientTimeMs }` | `{ user, clientTimeMs, serverTimeMs }` |
| `queue.remove` | `{ source, id }` | `{ source, id, name }` |
| `queue.like` | `{ source, id }` | `{ source, id, name, likes }` |
| `queue.get` | `{}` | `QueueData` |
| `playback.skipVote` | `{}` | `{ currentVotes, requiredVotes, skipped }` |
| `playback.get` | `{}` | `PlaybackData` |
| `settings.get` | `{}` | `{ playMode }` |
| `settings.set` | `{ playMode? }` | 完整设置；空补丁返回当前值 |
| `members.get` | `{}` | `{ users, onlineCount }` |

点歌使用 `POST /api/music/pick`，不存在 WebSocket `queue.enqueue` 请求。歌曲从开始完整播放到结束，不提供暂停、恢复或 seek；成员可通过投票切歌。`source` 为 `netease`、`qq`、`kuwo`，歌曲 ID 为字符串，时间和时长均为毫秒整数。聊天为 1–500 字符，昵称最长 80 字符，普通房间待播队列最多 10 首。

## 快照、版本与广播

加入房间后首先收到 `room.snapshot`：

```json
{
  "revision": 12,
  "room": { "id": "room-id", "name": "听歌房", "desc": "", "population": 1, "createTime": 123, "needPwd": false, "ultimate": false },
  "queue": { "current": null, "items": [], "onlineCount": 1 },
  "playback": { "song": null, "user": null, "status": "stopped", "startedAtMs": 0, "positionMs": 0, "serverTimeMs": 456 },
  "members": { "users": [], "onlineCount": 1 },
  "settings": { "playMode": "sequential" },
  "chat": [],
  "serverTimeMs": 456
}
```

快照已经包含播放、队列、成员和设置，客户端连接后不再重复发送 `playback.get`、`queue.get`、`members.get`、`settings.get`。`revision` 在每次状态变更后递增；各领域独立丢弃旧 revision，同一 revision 可同时包含队列与播放变化。收到 `sync.required` 后请求 `room.sync`。

| event | data |
| --- | --- |
| `room.snapshot` | 完整 `RoomSnapshot` |
| `queue.changed` | `{ revision, queue: QueueData }` |
| `playback.changed` | `{ revision, playback: PlaybackData }` |
| `members.changed` | `{ revision, members: MembersData }` |
| `settings.changed` | `{ revision, settings: SettingsData }` |
| `chat.created` | `{ revision, message: ChatData }` |
| `sync.required` | `{ reason }`，广播落后或需要全量恢复 |
| `room.closed` | `{ reason, message }`，租约失效或房间关闭 |
| `protocol.error` | `{ code, message }` |

`PlaybackData` 使用可推导锚点。`startedAtMs` 是服务器开始时间，`positionMs` 是根据服务器时间推导并截断到时长的进度。只在播放变更和重连快照时发送完整歌曲数据；轻量 `clock.ping` 用于时钟校准，不周期广播完整播放状态。节点恢复时自动跳过已经结束的曲目。

## 内部 RPC

Go `livehouse-api` 使用 `POST /rpc`，可配置 `Authorization: Bearer <INTERNAL_API_TOKEN>`。`LIVEHOUSE_API` 只是入口；实际节点以 Redis 心跳和归属表为准，RPC 自动转发到归属节点，Nacos 不参与房间路由。

响应为 `{ "version": 2, "data": ... }`，错误沿用 HTTP 状态码与 `{ "error": "..." }`。支持 `room.create`、`room.ensure`、`room.enter`、`room.list` 及房间操作，但不暴露给 gateway 外部客户端。

`queue.enqueue` 只允许内部 RPC 调用。livehouse-api 提交歌曲元数据，worker 不调用音乐提供方，也不保存点歌时的播放 URL 或响度系数：

```json
{
  "type": "queue.enqueue",
  "roomId": "room-id",
  "password": "",
  "data": {
    "playable": {
      "id": "123", "source": "qq", "name": "歌曲", "artist": "歌手", "album": "专辑",
      "durationMs": 180000, "pictureUrl": "", "webUrl": "", "url": "", "lyric": ""
    },
    "user": { "name": "API", "emailHash": "" },
    "likes": 0,
    "voters": []
  }
}
```

内部 JSON RPC 当前使用 HTTP keep-alive。若后续测量表明内部 RPC 成为瓶颈，可保持同一抽象操作模型，将传输替换为 gRPC + h2c 长连接和 Protobuf；公网 WebSocket 与 Redis 路由不需要改变，h2c 仅限可信内网。

## HTTP 边界

`/api/house/*` 与 `/api/music/*` 由 Go `livehouse-api` 提供。搜索和歌单接口返回 `{ items, totalSize }`，歌曲字段与协议 DTO 一致。`POST /api/music/recommend` 可带当前网易云歌曲 `id`，未提供时通过 RPC 从队列选择种子。点歌通过 HTTP 入队，广播仍由 worker 推送。

`POST /api/music/resolve` 接收 `{ source, id }`，使用 `UseCache: false` 获取最新播放 URL。前端在新播放锚点或音频 URL 失效时调用。相同 `(source,id)` 的并发请求由泛型 singleflight 合并，使用 `maphash` 生成 64 位 key；成功结果由 `RWMutex` TTL cache 保存 5 秒，到期后台删除，失败不缓存。响应可携带网易云提供的 `gain`（dB，含有效值 0），前端优先按 `10 ** (gain / 20)` 转换为音量系数；缺失时读取音频 ReplayGain 标签，再回退到默认系数 0.6。增益立即生效，并与用户音量相乘，最终音量上限为 1。worker 与前端均不依赖 loudness。

## 验证

```bash
cargo fmt --all -- --check
cargo test --workspace
cargo check --workspace --locked
go test ./...
cd frontend && npm run type-check && npm run lint && npm run build
```

v2 线上信封与 DTO 由 `livehouse-worker/src/protocol.rs`、`livehouse-worker/src/model.rs` 和 `frontend/src/types/livehouse-protocol.ts` 定义。旧 v1 协议、生成器与兼容映射已移除。
