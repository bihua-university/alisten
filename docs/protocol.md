# WebSocket protocol v1

后端 WebSocket 统一使用 `protocol` 包。`protocol/ws.idl.json` 是协议来源；`types.gen.go` 由生成器维护。信封层校验（version、kind、type、requestId 与已知消息类型）在 `DecodeRequest` 中完成，data 校验由各处理器自行解码 DTO 时负责。HTTP 接口保留原有请求与响应结构。

## 连接与消息

连接地址：`ws(s)://<host>/server?houseId=<encoded-id>&housePwd=<encoded-password>`。房间或密码错误时握手返回 404。

只接受 UTF-8 JSON 文本帧，最大 64 KiB。请求先经信封校验，再按协议常量分发；字段级校验由各请求处理器完成。每条连接最多同时处理 16 个请求，超出时返回 `RATE_LIMITED`。响应与广播可交错，不同请求可乱序完成。

请求：

```json
{"version":1,"kind":"request","type":"heartbeat","requestId":"r1","data":{"clientTimeMs":123}}
```

成功响应保留请求的 `type` 和 `requestId`：

```json
{"version":1,"kind":"response","type":"heartbeat","requestId":"r1","data":{"clientTimeMs":123,"serverTimeMs":456}}
```

失败响应的 `data` 为 `{}`，错误使用协议错误码：

```json
{"version":1,"kind":"response","type":"queue.remove","requestId":"r2","data":{},"error":{"code":"NOT_FOUND","message":"未找到要删除的音乐"}}
```

无法关联到已知请求类型和有效 ID 的错误，通过 `event / protocol.error` 返回。非法消息通常不会关闭连接；超出帧大小限制时关闭连接。

## 请求目录

| type | 请求 data | 成功响应 data |
| --- | --- | --- |
| `heartbeat` | `{ clientTimeMs }` | `{ clientTimeMs, serverTimeMs }` |
| `chat.send` | `{ content }` | `{}` |
| `user.update` | `{ name, email, clientTimeMs }` | `{ user, clientTimeMs, serverTimeMs }` |
| `settings.get` | `{}` | `{ playMode }` |
| `settings.set` | `{ playMode? }` | 完整的 `{ playMode }`；空补丁返回当前设置 |
| `members.get` | `{}` | `{ users, onlineCount }` |
| `music.search` | `{ source, keyword, pageIndex, pageSize }` | `{ items: Song[], totalSize }` |
| `playlist.search` | 同上 | `{ items: Playlist[], totalSize }` |
| `music.pick` | `{ source, id?, name? }`，ID/名称至少一个非空 | `{ source, id, name, artist }` |
| `queue.remove` | `{ source, id }` | `{ source, id }` |
| `queue.like` | `{ source, id }` | `{ source, id, likes }` |
| `playback.skipVote` | `{}` | `{ currentVotes, requiredVotes, skipped }` |
| `playback.get` | `{}` | `PlaybackData` |
| `queue.get` | `{}` | `QueueData` |
| `music.recommend` | `{}` | `{ items: Song[], totalSize }` |

`source` 统一为 `netease`、`qq`、`kuwo`、`url_common` 字符串；WebSocket 不接受 `wy` 别名或数字来源。来源枚举不保证每个 provider 支持全部操作；不支持或调用失败时返回 `PROVIDER_ERROR`。

搜索分页从 1 开始，`pageSize` 为 1–100。`music.search` 的 `keyword="*<playlistId>"` 返回歌单歌曲；当前为全量返回。聊天内容为 1–500 字符，空白内容拒绝；昵称空白也拒绝。其余约束以 Schema 为准。

队列歌曲按 `(source, id)` 定位。点赞和删除只操作等待队列；相同歌曲名、跨来源同 ID、点赞后重排都不会改变操作目标。重复点歌、队列已满、重复投票或没有可切换的下一首返回 `CONFLICT`。

## 广播与快照

| 事件 | data |
| --- | --- |
| `chat.message` | `{ content, user, sentAtMs }` |
| `members.updated` | `{ users, onlineCount }` |
| `settings.updated` | `{ playMode }` |
| `queue.updated` | `{ current: QueueItem \| null, items: QueueItem[], onlineCount }` |
| `playback.updated` | `{ song: PlayableSong \| null, user: User \| null, startedAtMs, volumeCoefficient }` |
| `notice` | `{ code, message }`，例如后台播放加载失败 |
| `protocol.error` | `{ code, message }` |

加入房间即收到播放、设置、队列、成员快照。成员进入或离开时更新成员和队列人数；修改设置后向所有成员广播完整设置。

空列表始终为 `[]`。空播放状态为 `{ song: null, user: null, startedAtMs: 0, volumeCoefficient: 1 }`。`current` 与等待队列 `items` 分开；准备中的歌曲暂不在等待队列中。没有下一首时保留最后播放的歌曲信息，客户端应结合时长停止播放。

歌曲时间字段为毫秒，来源和歌曲 ID 均为字符串；用户为 `{ name, emailHash }`。播放广播和查询响应使用同一份状态，包括歌曲、点歌用户、服务器开始时间及响度系数。聊天时间由服务器生成。客户端可使用心跳往返时间估计时钟偏移，再根据 `startedAtMs` 计算播放位置。

`requestId` 只用于关联，不提供幂等或重放去重。断线时清理待处理请求，重新连接后获取快照；写操作不要自动重发。

## 生成与验证

```bash
go generate ./protocol
go run ./cmd/protocolgen -check
go test ./...
```

只生成 Go 类型；`protocol` 包不包含生成的 TypeScript 文件。前端需要类型时，显式指定包外的输出路径（工具会自动创建目标目录）：

```bash
go run ./cmd/protocolgen -ts ../frontend/src/types/ws-protocol.ts
```

需要检查该输出时，使用相同的 `-ts` 路径并添加 `-check`。

后端路由和广播统一使用 `protocol` 常量；处理器读取协议 DTO 并在解码时完成字段级输入校验（长度、范围、枚举等业务约束），成功通过 `Context.Send`，失败通过 `Context.Fail`。新增协议时先修改 IDL、重新生成，再接入处理器并补充契约测试。

服务端契约用例位于 `cmd/alisten/testdata/websocket/*.txt` 和 `cmd/alisten/testdata/http/*.txt`，每个独立场景一份 txtar 文件，使用 `golang.org/x/tools/txtar` 读取。`-- setup --` 区块准备场景，`-- request --` 发起请求，`-- response --` 保存预期响应。区块正文记录实际协议报文，HTTP 请求正文前写 `POST /path`，响应正文前写状态码；格式示例见 `cmd/alisten/testdata/README.md`。

每个文件使用独立房间和连接，不依赖其他文件的执行顺序。HTTP 用例自动填入测试房间和密码，可在请求正文中覆盖。测试比较完整响应，并校验 WebSocket 响应帧的信封结构与关联 ID。

预期文件中，正数服务器时间戳统一写为 `1`；零值、客户端时间、时长及其他字段保留原值。协议行为有意变更时，审核并修改相应 golden 文件。运行 `go test ./cmd/alisten -run Golden` 检查契约；`go test ./...` 还包含协议边界校验及播放切换、房间关闭的并发回归。
