# 部署

运行两个 Go 服务：`alisten` 管理房间、队列和前端连接，`alisten-musicd` 提供音乐平台查询、播放链接、相似歌曲和响度分析。`musiclet-py` 是可选的通用 URL 下载 worker。

```text
浏览器 ── HTTP/WebSocket ── alisten ── protobuf RPC ── alisten-musicd
musiclet-py ── /tasks/* ── alisten ── HTTP 转发 ── alisten-musicd
```

歌曲信息中的 `url` 是源站播放链接。音频由浏览器直接访问。

## Docker / Podman Compose

准备配置：

```bash
cp config.json.example config.json
cp musicd-config.json.example musicd-config.json
cp musiclet-config.json.example musiclet-config.json
```

主服务的 `config.json` 使用 Compose 内的服务名：

```json
{
  "addr": ":8080",
  "debug": true,
  "token": "your-auth-token-here",
  "music": {
    "rpc": {
      "address": "alisten-musicd:9090",
      "http_url": "http://alisten-musicd:8081",
      "timeout_seconds": 210
    }
  },
  "persist": [{ "id": "room", "name": "音乐房间", "desc": "", "password": "" }]
}
```

`musicd-config.json` 存放平台 Cookie 和 worker 认证令牌：

```json
{
  "token": "your-auth-token-here",
  "debug": false,
  "music": { "cookie": "" }
}
```

`musiclet-config.json` 的 `server_url` 可设置为 `http://alisten:8080`，也可直接设置为 `http://alisten-musicd:8081`。它的 `token` 必须与 `musicd-config.json` 的 `token` 一致。下载、存储和数据库配置见 [worker 文档](../cmd/musiclet-py/README.md)。

启动 Go 服务：

```bash
docker compose up -d --build alisten alisten-musicd
# Podman 使用 podman-compose 执行相同命令
```

需要通用 URL 解析时，再启动 worker：

```bash
docker compose up -d --build musiclet-py
```

主服务对外监听 `8080`。musicd 的 `9090`（RPC）和 `8081`（worker HTTP）只在容器网络内使用。Compose 会等待 musicd 的 `/healthz` 健康检查通过后启动主服务。

Cookie 管理仍通过主服务的 `GET/POST /config/cookie`，使用主服务 `token` 进行 Bearer 认证。主服务通过 RPC 查询或更新 musicd 的 Cookie，更新结果中的 `persisted` 表示是否写入 musicd 配置文件。要持久化，容器用户 UID 1001 必须有写入该文件的权限；主服务配置可只读挂载。

## 本地 / 分机部署

使用 Go 1.27 或更新版本构建，普通构建无需安装 protoc：

```bash
go build -o alisten ./cmd/alisten
go build -o alisten-musicd ./cmd/alisten-musicd
```

本机部署时将主服务配置中的地址改为：

```json
"rpc": {
  "address": "127.0.0.1:9090",
  "http_url": "http://127.0.0.1:8081",
  "timeout_seconds": 210
}
```

分别启动：

```bash
./alisten-musicd -config musicd-config.json -listen 127.0.0.1:9090 -http 127.0.0.1:8081
./alisten
```

musicd 默认使用上述配置文件名和监听地址。分机部署时，设置 musicd 的 `-listen`、`-http` 和主服务的 `music.rpc.address`、`music.rpc.http_url` 为对应内网地址。RPC 使用内部可信网络。

主服务通过 RPC 库提供的 `rpc.Client` 连接 musicd：默认 8 个 TCP 连接槽位，按需建连、轮询分配请求，每个连接支持多个请求同时等待响应，连接池大小不限制请求并发数。连接中断后，仅对应槽位在下次使用时重连，失败请求不会自动重放。RPC 服务端并发处理不同连接和同一连接内的请求；响度分析通过 singleflight 合并同一歌曲的任务，并限制最多 4 个分析任务同时执行。

`timeout_seconds` 作为客户端的 `CallTimeout`，默认 210 秒，覆盖通用 URL worker 的三分钟任务等待和音频响度分析。RPC context 超时会结束客户端等待；当前框架不会把单次调用取消消息传到服务端。musicd 对每次处理设置独立的 210 秒期限，平台 HTTP 请求也有 30 秒超时。

`debug: true` 时主服务使用 HTTP `:8080`；`debug: false` 时 `addr` 应填写用于 CertMagic HTTPS 的域名。

## RPC IDL 和生成

两端仅共用 `internal/proto/musicd/musicd.proto` 与 `musicd.pb.go`。客户端实现位于 `cmd/alisten/internal/rpc/musicd`，只负责方法名，请求与响应直接使用生成的 protobuf 消息，连接池、按需建连和写超时由 RPC 库的 `rpc.Client` 处理；服务端及平台实现位于 `cmd/alisten-musicd/internal`。主服务不再维护重复的领域结构体，HTTP 旧接口的小写 JSON 字段由 `internal/proto/musicd/json.go` 的 `MarshalJSON` 保持。方法名使用 `musicd.v1.*`。

`go.mod` 的 `tool` 声明固定了 `protoc-gen-golite`，`require` 固定了 RPC 库的版本。修改 IDL 后，在仓库根目录安装 protoc 并直接生成。

Bash：

```bash
protoc --plugin=protoc-gen-golite="$(go tool -n protoc-gen-golite)" \
  --golite_out=. \
  --golite_opt=module=github.com/bihua-university/alisten \
  internal/proto/musicd/musicd.proto
```

PowerShell：

```powershell
$golite = go tool -n protoc-gen-golite
protoc --plugin="protoc-gen-golite=$golite" --golite_out=. --golite_opt=module=github.com/bihua-university/alisten internal/proto/musicd/musicd.proto
```

将 IDL 与生成文件一起提交。验证：

```bash
go test ./...
go vet ./...
```

## 运维

```bash
docker compose logs -f alisten alisten-musicd
docker compose ps
docker compose restart alisten-musicd
docker compose up -d --build alisten alisten-musicd
docker compose down
```

主服务仍在内存中保存房间及播放队列；重启 musicd 不会清空它们，musicd 的歌曲和响度缓存会重新建立。
