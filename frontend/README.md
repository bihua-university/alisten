# 同频（Alisten）

基于 Native Shell（NSH）和 TypeScript 的多人同步听歌应用，共享 Web 与 Android 界面和业务逻辑。应用入口和构建配置直接位于本目录，业务代码位于 `src/`。

本目录是 alisten monorepo 的前端部分。下文命令均在 `frontend/` 内执行；后端服务说明见仓库根目录的 README。

## 功能

- 房间搜索、创建、密码加入与邀请链接
- 网易云 / QQ 歌曲和歌单搜索、个性化推荐、点歌
- 同步播放、歌词、队列点赞 / 删除与切歌投票
- 聊天、在线成员、播放 / 搜索历史和导出
- 个人资料、Gravatar、外观、音量、性能与管理员 Cookie 设置
- 浏览器媒体会话和自动播放被拦截后的手动恢复入口
- Android 后台播放、音频焦点恢复、媒体通知和前台播放常亮

## 开发和构建

使用 Node.js 22。开发服务需安装 [NSH CLI](https://nsh.dev/docs/installation/)，Web 生产构建会自动准备固定版本 NSH 工具链。

```bash
npm ci
npm run dev
npm run type-check
npm run lint:fix
npm run lint
npm run build
npm run preview
```

`npm run dev` 在默认端口 8790 提供 NSH 热更新，`npm run build` 将生产站点写入 `dist/`，`npm run preview` 预览该目录。Vite 仅用于读取 `.env` 和预览静态文件；应用始终由 NSH 编译，不使用 React 或 React DOM。

Android 统一使用 `npm run build:android`，仅输出 ARM64 压缩 Release。宿主依赖、签名和后台播放说明见 [android/README.md](android/README.md)。

## 结构与运行机制

- `App.tsx`：加载跨端存储和 Web 配置，再初始化外观并挂载应用。
- `nsh.json`、`host.js`、`icon.png`：应用配置、Web 能力桥接和图标。
- `src/app.tsx`：导航、移动端 tab、房间切换与桌面右侧栏。
- `src/components/`：播放器、歌词、聊天、搜索、历史、房间和设置。
- `src/stores/`：模块级 EventTarget Store、播放时钟与 livehouse v2 同步协议。
- `src/hooks/use-store.ts`：将 Store 选择性订阅桥接为 NSH signal。
- `src/theme.ts`、`src/utils/ui-settings.ts`：语义颜色、字号与动效偏好。
- `scripts/`：Web 发布产物整理及固定版本工具链准备。
- `android/`：Alisten 的 Kotlin 宿主、媒体会话与前台播放服务。
- `.nsh/`：自动生成的声明、缓存及原生构建产物，不提交 Git。

组件只初始化一次，动态数据通过 signals / effects 更新。播放进度不重建页面，歌词只更新当前行；视觉更新在后台暂停。封面各自加载，搜索内容可渐进显示。性能设置提供高质量、平衡、省电和极简四级降级。

WebSocket 使用无限指数退避重连、心跳校时和回前台探活。播放位置由服务器开始时间推导；加入、重连和恢复播放时重新定位，相同歌曲快照复用已加载音频。浏览器以实际 `play()` 结果确认播放；被拦截时显示恢复提示，关闭提示后底部仍保留播放按钮。

## Cloudflare Pages

可继续使用原来的 Pages 项目和域名，构建配置保持：

| 配置 | 值 |
| --- | --- |
| 根目录 | `frontend` |
| 构建命令 | `npm run build` |
| 输出目录 | `dist` |
| Node.js | `22`（`.node-version`） |

如果原构建命令是 `npx vite build`，需改为 `npm run build`。框架预设可使用 None，构建和输出设置以本表为准。

Web 生产构建继续读取 `VITE_WS_URL`、`VITE_API_URL`、`VITE_APP_NAME`，控制台环境变量优先于 `.env.production.local`、`.env.production`、`.env.local` 和 `.env`。这些变量公开写入网页，不应存放密钥。其他环境可使用 `npm run build -- --mode staging`。Android 和直接 `nsh run` 的默认配置在 `src/utils/config.ts`。

构建脚本校验官方 NSH 0.4.0 工具链的 SHA-256，只提取 CLI 和 Web runtime。Linux 镜像如不满足 glibc 要求，会在缓存中提取私有 loader，无需更换系统库或使用 sudo。首次下载需要访问 GitHub Releases 和 Ubuntu archive。已有 `.nsh/toolchains/` 会被复用，也可用 `NSH_BIN` 指定同版本 CLI 的绝对路径。

`dist/` 包含 HTML、JS、WASM、图标、manifest 和 Cloudflare `_headers`，上传整个目录即可。NSH 的压缩副本不上传，由 Pages 压缩响应；入口与 manifest 重新验证，WASM 使用正确的 MIME 类型。

不实现 PWA 离线缓存和更新提示。保留的 `cloudflare/sw.js` 只用于注销线上旧 worker、清理对应 scope 的旧 precache，并刷新页面一次，避免旧版本缓存拦住新站点；它不清除设置和历史记录。

## 验证与协作

运行类型检查、Lint 和目标平台构建；界面通过实际浏览器人工检查，不新增 UI 单元测试或布局断言。实际音乐提供方的可用性需在服务端房间中联调。

界面规范见 [DESIGN.md](DESIGN.md)，工程约定见 [AGENTS.md](AGENTS.md)，线上协议见 [protocol.md](../docs/protocol.md)。
