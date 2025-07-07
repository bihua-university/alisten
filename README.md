## Alisten

一个音乐播放和房间管理系统，支持从 Bilibili、网易云音乐、QQ音乐等平台获取音乐。

## Config

put config.json in the root directory of your project.

```json
{
    "addr": ":80",
    "music": {
        "netease": "...",
        "cookie": "...",
        "qq": "..."
    },
    "qiniu": {
        "ak": "",
        "sk": ""
    },
    "debug": true,
    "pgsql": "...",
    "persist": [
        {
            "id": "room1",
            "name": "音乐房间1",
            "desc": "这是一个持久化的音乐房间",
            "password": "123456"
        },
        {
            "id": "room2", 
            "name": "音乐房间2",
            "desc": "另一个持久化房间",
            "password": ""
        }
    ]
}
```

### 配置说明

- `addr`: 服务器监听地址
- `music.netease`: 网易云音乐 API 地址
- `music.cookie`: 音乐平台 Cookie
- `music.qq`: QQ音乐 API 地址
- `qiniu.ak`: 七牛云 Access Key
- `qiniu.sk`: 七牛云 Secret Key
- `debug`: 调试模式开关
- `pgsql`: PostgreSQL 数据库连接字符串
- `persist`: 持久化房间配置数组
  - `id`: 房间唯一标识符
  - `name`: 房间显示名称
  - `desc`: 房间描述
  - `password`: 房间密码（可选，为空表示无密码）

## Features

- 🎵 支持多平台音乐源（Bilibili、网易云音乐、QQ音乐）
- 🏠 房间管理系统，支持持久化房间配置
- 🎶 HTTP API 点歌功能，支持通过 REST API 进行点歌

## API 接口

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
    "code": "20000",
    "message": "点歌成功",
    "data": {
        "name": "音乐名称",
        "source": "wy",
        "id": "音乐ID"
    }
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
go build && ./alisten
```
