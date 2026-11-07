# AGENTS.md

本文件用于指导「人类贡献者」与「AI 编程代理」在本仓库内协作时的默认行为、工程约束与常用入口。

---

# 仓库协作指南

## 项目定位

**同频**（Alisten）是一个实时在线听歌房 Web 应用。多人在同一房间同步播放、点歌、聊天，支持 Web 与 Android、歌词同步和浏览器媒体会话。

---

## 技术基线

- **前端框架**：Native Shell（NSH）0.4+，同一套 TSX 支持 Web、Windows、Android
- **开发语言**：TypeScript 5.3+，NSH signals / effects，不使用 React DOM
- **构建工具**：`nsh` CLI（[安装文档](https://nsh.dev/docs/installation)）
- **样式**：NSH typed styles 与原生 Tailwind classes；设计 token 在 `src/theme.ts`
- **图标**：`src/assets/icons/` 中随应用打包的 SVG，由 `components/icon.tsx` 统一引用
- **状态管理**：模块级 EventTarget Store，组件通过 `useStore` 桥接为 NSH signal
- **工具库**：`crypto-js`、`music-metadata`
- **Node.js 版本**：22.x（`.node-version`）；用于依赖、Web 构建脚本、TypeScript 与 ESLint
- **唯一前端**：业务实现位于 `src/`；Vite 仅用于读取 `.env` 与静态产物预览，不编译应用

---

## 项目结构（重要入口）

```
frontend/
├── App.tsx                    # 初始化跨端存储与浏览器桥接，挂载应用
├── nsh.json                   # 应用 ID、图标、能力与 Web host 配置
├── host.js                    # Web 标题、分享、媒体会话、旧 localStorage 迁移
└── src/
    ├── app.tsx                # 主界面、导航、房间切换与右侧栏
    ├── theme.ts               # 语义颜色、字号、动效与响应式参数
    ├── hooks/use-store.ts     # EventTarget Store → NSH signal
    ├── stores/                # 保留同步播放与 livehouse v2 协议
    ├── components/            # 歌词、搜索、队列、聊天、历史、设置等
    ├── types/                 # UI 类型与 livehouse-protocol.ts v2 协议类型
    ├── utils/                 # API、用户、时间、跨端存储与运行时桥接
    └── assets/icons/          # 打包 SVG 图标
```

### 构建产物

- Web：`npm run build` 输出 `dist/`，兼容 Cloudflare Pages；原生 `nsh package` 仍输出 `.nsh/out/同频/`
- Windows：`.nsh/out/同频/` 与同名 `.zip`
- Android：仅 ARM64 压缩 Release，输出 `.nsh/out/alisten-android-release/Alisten-<version>-arm64-v8a-compressed-release.apk`

---

## 开发 / 构建 / 检查命令

```bash
# 安装依赖与 NSH CLI（https://nsh.dev/docs/installation）
npm install

# Web 热更新（默认 http://127.0.0.1:8790）
npm run dev

# 类型检查与代码检查
npm run type-check
npm run lint
npm run lint:fix

# Web 生产构建与预览
npm run build
npm run preview

# Windows / Android
nsh package App.tsx --target windows
npm run build:android

```

**质量门槛**：只要改动涉及逻辑/类型/样式，合并前至少跑 `npm run type-check` 和 `npm run lint`。默认构建必须通过；目标平台改动还需补对应的 `nsh package`。可用 `nsh test App.tsx` 做无界面启动检查，但不新增 UI 测试文件。

### 测试约定

- 不编写或新增 UI、前端单元测试。
- 尤其不要为界面数值、尺寸、间距、坐标、断点或 layout（布局）编写断言，也不要通过组件测试、快照测试或浏览器自动化测试变相添加这些断言。
- UI 与布局改动通过实际浏览器人工检查，代码质量继续使用类型检查、Lint 和必要的构建检查验证。

---

## UI / 样式规范

视觉和工程规范以 `DESIGN.md` 为准。不显示左侧栏，导航和同步播放收进顶部菜单；顶部参考 Spotify 搜索栏，内容和右侧采用独立面板。听歌房主界面仅显示歌词，当前行放大并高亮，切换带过渡；默认关闭右侧面板。右侧面板桌面宽度 480px，紧凑桌面 420px。底部播放器左侧封面，主体歌曲信息和宽进度条，时间靠右；右侧保留切歌、音量和辅助入口。中间歌曲信息不能点击。主内容和右侧面板无外边框；顶部栏和底部播放器贴边，无边框、圆角、外侧留白或毛玻璃，背景固定 #000000。

- 使用语义颜色 token、统一 `text-ui-*` 字号、系统字体和明确的文字层级。
- 点击顶部搜索展开平台 / 类型选择浮窗，提交后进入搜索页；不提供独立点歌导航 tab。
- 普通控件圆角 8px，内容面板 12px，弹窗 16px；主题、对比度和键盘焦点都要验证。
- 高频进度通过 NSH `AnimatedValue` 以最高 30 FPS 更新 `scaleX`，不使用连续原生 timing 或 JS 逐帧循环；后台停止视觉更新，时间只在秒数变化时更新，歌词只更新当前行。
- 页面按需加载，列表减少屏外绘制；新增效果必须有性能降级。

---
## 代码与模块约定

### NSH 组件
- 组件函数只运行一次；动态值必须在 JSX binding、`computed` 或 `effect` 中读取 signal。
- 使用 `signal` 管理局部交互，避免把播放进度、滚动等连续值放入组件树重建路径。
- 在 `effect` 中建立订阅，并用返回值或 `cleanup` 清理事件、定时器和 rAF。
- 使用 `View`、`Text`、`Scroll`、`Image`、`Video` 与 `nsh/base` 控件；文本只能放在 `Text` 或 `Input` 中，不写 HTML 小写标签。
- 媒体 `onLoad` 对应每次新源就绪；重复播放快照不能重启音频。
- 不直接访问 `window`、`document`、DOM 或 `localStorage`；跨端存储走 `utils/storage.ts`，Web 专属能力走 `utils/runtime.ts` 与 `host.js`。

### 状态管理（Store 模式）
- 所有全局状态通过 `src/stores/` 中的 Store 类管理。
- Store 基类继承 `EventTarget`，通过 `dispatchEvent('change')` 通知订阅者。
- 组件通过 `useStore(store, selector)` 取得 signal；音频层直接订阅 Store 并清理。
- 避免在组件间直接传递复杂状态。

### TypeScript
- 严格模式：`strict: true`。
- 路径别名：`@/` 映射到 `src/`，同时配置在 `nsh.json` 与 `tsconfig.json`。

### ESLint 风格规范

项目使用 `@antfu/eslint-config`，核心风格规则如下：

- **引号**：字符串统一使用单引号（`'`）
- **分号**：禁止语句末尾分号
- **if 换行**：`if` 语句的条件表达式后必须换行，如 `if (condition)\n  return`
- **运算符换行**：二元运算符（`=`、`||`、`>=` 等）必须放在行首
- **箭头函数括号**：单参数且函数体无大括号时省略括号，如 `x => x + 1`
- **缩进**：2 空格缩进，二元运算操作数对齐特定规则
- **大括号风格**：使用 1tbs（`style/brace-style`）

提交前务必运行 `npm run lint:fix` 自动修复格式问题。

### 命名约定
- 组件文件：kebab-case（如 `chat-panel.tsx`）
- NSH 组件名：PascalCase（如 `ChatPanel`）
- 组合式函数/Store：`useXxx` / `XxxStore`（如 `playerStore`）
- 工具函数：camelCase

---

## WebSocket 通信架构

NSH 的 WebSocket 与 REST 默认地址集中在 `src/utils/config.ts`，Web 生产构建支持环境变量覆盖。Rust livehouse 是唯一房间状态持有者，gateway 按 Redis 房间归属转发；Go livehouse-api 只保留 HTTP 兼容 API。

### 连接方式
- 连接 URL 格式：`${wsUrl}/ws?roomId=${roomId}&password=${password}`
- 自动重连：**指数退避且不设次数上限**，3s → 6s → 12s → 24s → 30s(封顶)。不要改回「固定次数后放弃」，移动端切后台会导致永久失联
- 心跳间隔：5 秒，发送 v2 `clock.ping` 请求并携带 `clientTimeMs`；服务端响应用于估计时钟偏移，同时兼作保活帧
- **不要**仅凭「未收到心跳响应」判断连接死活。失效检测依赖死连接 write 触发的 `onerror`/`onclose`，回前台时检查 `readyState`
- 自愈触发点：NSH `lifecycle()`、Web host 的 `online` / `pageshow` 事件。已有连接在回前台时先探活，避免无谓重连

### 消息格式（livehouse v2）

线上信封 `{ version: 2, kind, type, requestId?, data }` 与 DTO 定义于 `src/types/livehouse-protocol.ts`、`livehouse-worker/src/protocol.rs` 和 `livehouse-worker/src/model.rs`。旧 v1 协议与生成器已移除。

- 请求与响应通过 `requestId` 关联，事件无 `requestId`；所有业务数据放在 `data`
- 来源为 `netease` / `qq` / `kuwo`，ID 为字符串，时间和时长为毫秒整数
- 加入后先接收 `room.snapshot`；`queue.changed`、`playback.changed`、`members.changed`、`settings.changed`、`chat.created` 携带 `revision`
- 各领域独立丢弃旧 revision；同一 revision 可同时包含队列和播放变化。收到 `sync.required` 时用 `room.sync` 获取完整快照

### 消息路由机制

- Store 直接调用 `websocketStore.request(type, data)` 发送 v2 请求；该层处理 pending、超时与断线清理，写操作不自动重放
- Store 通过 `registerEventHandler` 订阅内部 DTO 事件；v2 的 snapshot/changed 事件由 WebSocket 层转换为播放、队列等局部快照后分发
- `protocol.error` 与失败响应统一弹 Toast；不得解析文案判断业务分支
- 实时请求仅包括队列、投票切歌、聊天、成员、设置和播放同步。不提供暂停、恢复或 seek
- 点歌、搜索、歌单与推荐统一走 `utils/api.ts` 的 HTTP 请求，均不依赖 WebSocket 连接状态
- 加入后的 `room.snapshot` 已包含播放、队列、成员和设置，不要在每次连接后重复请求这四类快照；没有周期 `playback.sync` 大快照

---

## 播放器同步机制

- `startedAtMs`：当前歌曲开始播放的服务器时间戳，来自初始快照或播放变更事件
- 「正在播放」只有 `currentSong` 一个来源，取自播放快照；`queue.updated` 只替换等待队列，不能用队列的 `current` 覆盖播放状态
- `clockOffsetMs`：由心跳及用户更新响应估计，优先取往返时延最小的样本
- 播放位置按 `Date.now() + clockOffsetMs - startedAtMs` 推导，clamp 到 `[0, durationMs]` 后除以 1000 写入音频；服务端不提供暂停和 seek
- 播放开始时间可能领先约 200ms：负数进度应等待开始，不要误判为时钟错误
- 按 `durationMs` 停止播放，不要把非空 `song` 当作「仍在播放」的标志
- UI `Song.duration` 全程为毫秒；进度使用 `requestAnimationFrame` + NSH `Video.onProgress` 双循环
- NSH `Video` 负责跨端音频输出，`paused`、`muted`、`volume` 由 Store 驱动，ref 仅用于同步 seek

---

## 移动端适配策略

- **设备检测**：`compact()` 读取响应式 `windowSize()`，767px 及以下使用移动布局，不做 UA 嗅探
- **安全区**：根布局、移动导航与弹窗使用 `env(safe-area-inset-*)` 或 `safeAreaInsets()`，桌面端为 0
- **视口高度**：根视图使用 `flex: 1` 填满 NSH surface；滚动只发生在 `Scroll` / `List` 容器内
- **移动导航**：底部依次为聆听、搜索、待播、聊天，内容按 tab 方向平移；顶部移除 Home 和搜索，保留更多、返回、房间状态与设置。历史和房间在顶部菜单。
- **返回键处理**：NSH `Dialog` / `Overlay` 原生处理 Escape、浏览器 Back 和 Android Back

---

## 性能优化体系

### 四级性能模式
`performanceStore` 管理四级性能设置；组件通过 `motionDuration()` 和 `reduceMotion()` 读取实际偏好，不再向 `document.body` 写 class：

| 级别 | 行为 |
|------|------|
| `high` | 完整交互过渡、封面阴影与局部模糊 |
| `medium` | 平衡设置，保留完整歌词过渡 |
| `low` | 缩短动画、立即定位歌词，保留必要的透明度过渡 |
| `off` | 极简模式：禁用动画与平滑滚动，保留静态大小和高亮层级 |

### 移动端效果收敛

- 不使用持续背景动画或大面积背景光晕；顶部栏和底部播放器不使用模糊。
- 极简与减少动态效果禁用动画及平滑滚动。歌词滚动必须读取实际性能偏好。
- 新增效果需覆盖 high / medium / low / off，并遵循 NSH 原生动画能力。

---

## Web 静态站点与旧缓存迁移

- NSH Web 构建生成 `manifest.webmanifest` 与安装图标，静态资源带内容摘要，可部署到任意静态托管。
- 不实现 Service Worker 离线缓存和更新提示。`cloudflare/sw.js` 仅用于注销线上遗留的旧 worker、移除对应 scope 的 precache，防止旧缓存继续拦截新站点。

---

## 环境变量

`npm run build` 通过 `scripts/build-web.mjs` 读取原有 `VITE_WS_URL`、`VITE_API_URL`、`VITE_APP_NAME` 并通过 Web host 注入 NSH；支持 Vite `.env` 文件和控制台环境变量优先级。Android 和直接 `nsh run` 不读取 Vite 环境变量，其默认配置仍在 `src/utils/config.ts`。Cloudflare 配置见 `README.md`。

---

## Git 约定

提交信息使用简洁的 Conventional Commits 风格：
```
feat: 新功能
fix: 修复问题
docs: 文档更新
style: 样式调整
refactor: 代码重构
perf: 性能优化
```

---

## AI 代理工作方式（默认行为）

- **先读后改**：优先定位相关组件/Store/样式入口
- **变更最小化**：不做无关重命名、不做大面积格式化
- **UI 变更要对齐项目规范**：DESIGN.md 中的语义颜色、音乐界面层级和一致的交互态
- **不编辑生成物或依赖目录**：不要改 `dist/`、不要改 `node_modules/`
- **状态管理注意**：全局状态分布在各 Store 中，修改时注意是否影响其他组件
- **性能敏感**：新增动画/效果时，考虑通过 `performanceStore` 添加对应级别的降级规则
- **移动端优先**：任何 UI 改动需同时考虑桌面端和移动端体验

---

*最后更新：2026年11月7日*
