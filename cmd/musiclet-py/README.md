# Python版本Musiclet

## 简介

这是基于Python重写的musiclet，使用yt-dlp下载音频，专门处理`url_common:get_music`任务类型。

## 功能特性

- ✅ 使用yt-dlp支持多种平台的音频下载（YouTube、Bilibili、Twitter等）
- ✅ 自动下载并上传缩略图到存储服务
- ✅ 支持S3存储上传（音频和缩略图）
- ✅ PostgreSQL数据库存储音乐信息
- ✅ 异步任务处理
- ✅ 完善的错误处理和日志记录

## 依赖项

安装所需的Python包：

```bash
pip install -r requirements.txt
```

或者手动安装依赖：

```bash
pip install aiohttp psycopg2-binary yt-dlp boto3 python-dateutil
```

## 配置

1. 复制配置文件模板：
```bash
cp config.json.example config.json
```

2. 编辑`config.json`配置文件：
```json
{
  "server_url": "https://your-server.com",
  "token": "your-auth-token-here",
  "storage": {
    "type": "s3",
    "s3": {
      "access_key_id": "your-s3-access-key",
      "secret_access_key": "your-s3-secret-key",
      "region": "us-east-1",
      "bucket": "your-s3-bucket",
      "endpoint_url": "https://s3.amazonaws.com"
    }
  },
  "pgsql": "postgresql://username:password@localhost:5432/database"
}
```

## 使用方法

```bash
cd cmd/musiclet-py
python main.py
```

## 测试缩略图功能

可以使用提供的测试脚本来验证缩略图下载功能：

```bash
cd cmd/musiclet-py
python test_thumbnail.py
```

这个脚本会测试下载一个YouTube视频的音频和缩略图，并显示相关信息。

## 任务类型

支持的任务类型：

- `url_common:get_music`: 通用URL音频下载任务
  - 参数: `url` - 要下载的音频/视频URL
  - 返回: 包含音乐信息的JSON对象

## 项目结构

```
cmd/musiclet-py/
├── main.py              # 主程序入口
├── config.py            # 配置管理
├── task.py              # 任务和结果数据结构
├── client.py            # 任务客户端（HTTP通信）
├── downloader.py        # 音频和缩略图下载、存储上传
├── database.py          # 数据库操作
├── config.json.example  # 配置文件模板
├── test_thumbnail.py    # 缩略图功能测试脚本
├── requirements.txt     # 依赖清单
└── README.md           # 说明文档
```

## 与Go版本的差异

1. **下载引擎**: 使用yt-dlp替代特定的bilibili下载逻辑
2. **任务类型**: 处理`url_common:get_music`而非`bilibili:get_music`
3. **多平台支持**: 支持YouTube、Bilibili、Twitter等多种平台
4. **缩略图处理**: 自动下载缩略图并上传到存储服务
5. **异步处理**: 使用asyncio进行异步任务处理

## 日志

程序运行时会生成两种日志：
- 控制台输出：实时查看程序状态
- 文件日志：保存到`musiclet.log`文件

## 错误处理

程序包含完善的错误处理机制：
- 网络请求失败自动重试
- 下载失败时清理临时文件
- 数据库操作异常处理
- 优雅的程序退出处理

## 注意事项

1. 确保PostgreSQL数据库可访问
2. 配置正确的S3存储服务凭据
3. 网络环境需要能访问目标音频/视频平台
4. 建议在稳定的网络环境下运行
5. 支持所有S3兼容的存储服务（AWS S3、MinIO、阿里云OSS等）