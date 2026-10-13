# 服务端 golden 测试

每个独立场景一份 `.txt`，按 `websocket/` 和 `http/` 分类，使用 [txtar](https://pkg.go.dev/golang.org/x/tools/txtar) 格式。文件开头可写场景说明，随后按区块顺序执行：

- `-- setup --`：前置请求，必须成功，忽略其响应正文。
- `-- request --`：待验证的请求。
- `-- response --`：完整的预期响应；WebSocket 也可用来检查事件。

同一文件内可以重复这些区块，组成完整场景。每个文件独立创建房间和连接，可以单独运行，不依赖其他场景。

## WebSocket 示例

```text
心跳回显客户端时间，并返回服务器时间。
-- request --
{"version":1,"kind":"request","type":"heartbeat","requestId":"heartbeat","data":{"clientTimeMs":123}}
-- response --
{"version":1,"kind":"response","type":"heartbeat","requestId":"heartbeat","data":{"clientTimeMs":123,"serverTimeMs":1}}
```

正文为真实 WebSocket 消息。以请求或 setup 开头的场景跳过加入房间时的初始快照；`connection.txt` 直接以 response 开头，验证四种初始快照。所有收到的消息均校验协议 Schema，响应同时检查 type 和 requestId。

## HTTP 示例

```text
HTTP 播放模式仍使用 mode 字段。
-- request --
POST /music/playmode
{"mode":"random"}
-- response --
200
{"mode":"random"}
```

请求区块首行为方法和路径，正文为请求体。测试自动填入当前房间的 houseId 和 password；正文显式提供这些字段时覆盖默认值。响应区块首行为状态码。

比较时忽略 JSON 对象字段顺序，正数服务器时间戳归一为 `1`；零值、客户端时间、时长及其他字段保持原值。修改接口行为时，审核并更新相应预期区块。

```bash
go test ./cmd/alisten -run Golden
go test ./cmd/alisten -run TestWebsocketGolden/queue
```
