# 壁画音乐厅（Alisten）

基于 React 18.3、TypeScript、Vite 和 TailwindCSS 的多人同步听歌应用。界面结合 Apple Music 的音乐层级和 Spotify 的搜索 / 面板布局，支持桌面底部播放器和移动端常驻迷你播放器。

> 本目录是 [alisten](https://github.com/bihua-university/alisten) monorepo 的前端部分，由原 `alisten-ui` 仓库整体导入，不以 submodule 引入。下文命令均在 `frontend/` 目录内执行；后端服务与部署说明见仓库根目录的 README 和 `docs/deploy.md`。

## 功能

- 房间搜索、创建、密码加入与邀请链接
- 网易云 / QQ 歌曲和歌单搜索、个性化推荐、点歌
- 同步播放、歌词、队列点赞 / 删除与切歌投票
- 聊天、在线成员、播放 / 搜索历史和导出
- 个人资料、Gravatar、外观、音量、性能与管理员 Cookie 设置
- PWA 安装、更新检测与浏览器媒体会话

## 开发

```bash
npm install
npm run dev
npm run type-check
npm run lint:fix
npm run lint
npm run build
npm run preview
```

环境变量：`VITE_WS_URL`、`VITE_API_URL`、`VITE_APP_NAME`。WebSocket 使用 v1 协议，见 `websocket-v1-migration.md`。

## 架构与性能

- `src/main.tsx`：React 入口和性能 / 外观初始化。
- `src/app.tsx`：顶部菜单、搜索入口、底部播放器、页面懒加载和房间切换。
- `src/components/`：播放器、歌词、聊天、搜索、历史、房间和设置。
- `src/components/audio-engine.tsx`：音频和连接生命周期。
- `src/hooks/use-store.ts`：用 `useSyncExternalStore` 选择性订阅不可变 Store 状态。
- `src/stores/`：保留模块级 EventTarget Store 和原有播放时钟 / WebSocket 架构。
- `src/style.css`：Apple Music 风格、语义颜色、字号 token 和响应式布局。
- `src/styles/performance.css`：四级性能与减少动态效果规则。

播放进度保持约 10fps 的 DOM 更新，不触发整页 React 渲染；歌词只更新当前行。搜索、设置、历史等页面懒加载，封面延迟加载，长列表减少屏外绘制。WebSocket 无限指数退避、心跳校时、网络恢复和回前台探活机制保持不变。

`canplay` 初始化每次音频加载仅执行一次，避免 seek 触发重复同步。相同播放广播不重新启动音频；显式恢复播放会请求最新位置。未来开始时间等待后播放，已播完的元信息保留但音频暂停。

界面和工程规范见 `DESIGN.md`；协作说明见 `AGENTS.md`。

## 浏览器验证

UI 与布局通过实际浏览器人工检查，覆盖桌面与移动端、深浅主题、键盘操作和主要交互流程。实际音乐服务的资源可用性需在已配置的服务端房间中联调。测试约定见 `AGENTS.md`。

## 许可证

MIT，见 `LICENSE`。
