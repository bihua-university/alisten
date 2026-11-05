# AGENTS.md

本文件用于指导「人类贡献者」与「AI 编程代理」在本仓库内协作时的默认行为、工程约束与常用入口。

---

# 仓库协作指南

## 项目定位

**壁画音乐厅**（Alisten）是一个实时在线听歌房 Web 应用。多人在同一房间同步播放、点歌、聊天，并支持歌词同步与 PWA 安装。

---

## 技术基线

- **前端框架**：React 18.3+
- **开发语言**：TypeScript 5.3+
- **构建工具**：Vite 5.0+
- **CSS 框架**：TailwindCSS 3.4+
- **图标库**：`lucide-react`（按需导入）
- **状态管理**：模块级 Store 类（EventTarget-based，零外部依赖）
- **工具库**：`crypto-js`
- **PWA**：`vite-plugin-pwa` + Workbox
- **Node.js 版本**：建议 >= 18（CI 使用 22.x）

---

## 项目结构（重要入口）

```
src/
├── main.tsx                   # React 入口，初始化性能与外观
├── app.tsx                    # 主界面、导航、懒加载、房间切换
├── hooks/use-store.ts         # useSyncExternalStore 字段选择器
├── stores/                    # EventTarget Store，保留同步播放与 livehouse v2 协议
│   ├── player-store.ts
│   ├── websocket-store.ts
│   ├── appearance-store.ts    # 主题与 --ui-font-size
│   └── ...                    # 聊天、歌词、历史、房间、性能、PWA 等
├── components/
│   ├── audio-engine.tsx       # 音频、连接与媒体会话生命周期
│   ├── header-search.tsx      # 顶部搜索与选择浮窗
│   ├── player-bar.tsx         # Spotify 式底部播放器，播放控制仅切歌
│   ├── listen-page.tsx        # 听歌房歌词主界面
│   ├── room-browser.tsx       # 搜索、创建、加入房间
│   ├── search-page.tsx        # 搜索提交后的结果页
│   ├── queue-page.tsx
│   ├── chat-panel.tsx
│   ├── lyrics.tsx             # 当前行高亮，不重建歌词树
│   ├── history-page.tsx
│   ├── settings-modal.tsx
│   ├── notifications.tsx
│   └── ui.tsx                 # 公共控件与原生 dialog
├── types/                     # UI 类型与 livehouse-protocol.ts v2 协议类型
├── utils/                     # API、用户、时间、移动端、弹窗历史等
├── style.css                  # 语义颜色、字号、布局
└── styles/performance.css     # 四级性能覆盖规则
```

### 构建产物

- `dist/` — Vite 生产构建输出目录
- PWA 产物：`sw.js`、`workbox-*.js`、`manifest.webmanifest`

---

## 开发 / 构建 / 检查命令

```bash
# 安装依赖
npm install

# 启动开发服务器
npm run dev

# 类型检查
npm run type-check

# 代码检查
npm run lint

# 代码检查并自动修复
npm run lint:fix

# 生产构建
npm run build

# 预览构建产物
npm run preview
```

**质量门槛**：只要改动涉及逻辑/类型/样式，合并前至少跑 `npm run type-check` 和 `npm run lint`。若改动影响构建路径（PWA、Vite 配置等），再补一次 `npm run build`。

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
- 高频进度通过 ref 更新 transform，时间只在秒数变化时更新；歌词只更新当前行。
- 页面按需加载，列表减少屏外绘制；新增效果必须有性能降级。

---
## 代码与模块约定

### React
- 使用函数组件和 TypeScript TSX，外部状态通过 `useStore` 选择稳定字段或数组。
- 用 `useState` 管理局部交互，避免把播放进度、滚动等连续值放入 React state。
- 在 `useEffect` 中建立订阅，并清理事件、定时器和 rAF。
- `canplay` 每次加载只监听一次，重复播放快照不能重启音频。
- 组件使用普通 DOM，公共控件复用 `components/ui.tsx`，不使用 Shadow DOM。
### 状态管理（Store 模式）
- 所有全局状态通过 `src/stores/` 中的 Store 类管理
- Store 基类继承 `EventTarget`，通过 `dispatchEvent('change')` 通知订阅者
- React 组件通过 `useStore(store, selector)` 订阅更新；音频层直接订阅 Store 并清理
- 避免在组件间直接传递复杂状态

### TypeScript
- 严格模式：`strict: true`、`noUnusedLocals: true`、`noUnusedParameters: true`
- 路径别名：`@/` 映射到 `src/`

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
- React 组件名：PascalCase（如 `ChatPanel`）
- 组合式函数/Store：`useXxx` / `XxxStore`（如 `playerStore`）
- 工具函数：camelCase

---

## WebSocket 通信架构

WebSocket 服务器地址通过环境变量 `VITE_WS_URL` 配置。Rust livehouse 是唯一房间状态持有者，gateway 按 Redis 房间归属转发；Go livehouse-api 只保留 HTTP 兼容 API。

### 连接方式
- 连接 URL 格式：`${wsUrl}/ws?roomId=${roomId}&password=${password}`
- 自动重连：**指数退避且不设次数上限**，3s → 6s → 12s → 24s → 30s(封顶)。不要改回「固定次数后放弃」，移动端切后台会导致永久失联
- 心跳间隔：5 秒，发送 v2 `clock.ping` 请求并携带 `clientTimeMs`；服务端响应用于估计时钟偏移，同时兼作保活帧
- **不要**仅凭「未收到心跳响应」判断连接死活。失效检测依赖死连接 write 触发的 `onerror`/`onclose`，回前台时检查 `readyState`
- 自愈触发点：`visibilitychange` / `online` / `pageshow`。已有连接在回前台时先探活，避免无谓重连

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
- UI `Song.duration` 全程为毫秒；进度更新使用 `requestAnimationFrame` + `audio.timeupdate` 双循环

---

## 移动端适配策略

- **设备检测**：`isMobileDevice()` 用媒体查询组合「窄视口 (`max-width: 767px`) + 粗指针 (`pointer: coarse`)」，不再做 UA 嗅探（UA 嗅探识别不了上报为 macOS 的 iPad）。需要随断点变化时用 `onMobileDeviceChange()`
- **安全区**：`index.html` 的 viewport 带 `viewport-fit=cover`，因此全屏容器需要自行补 `env(safe-area-inset-*)`（主布局与两个全屏弹窗已处理）。桌面端 env() 为 0，可无条件书写
- **视口高度**：优先用 CSS `100dvh`（`.h-screen-mobile`、`.app-viewport`），CSS 变量 `--vh` 仅作为旧浏览器兜底，由 `setViewportHeight()` 以 rAF 合并并跳过重复值写入
- **滚动控制**：全局阻止 `body` / `html` 滚动，仅允许特定容器滚动
- **移动导航**：底部导航切换聆听、队列、聊天、历史和房间；搜索位于顶部
- **返回键处理**：`modal-history.ts` 维护 dialog 堆栈，返回键仅关闭顶部弹窗

---

## 性能优化体系

### 四级性能模式
`performanceStore` 管理四级性能设置，通过 `document.body` 上的 class 控制全局样式：

| 级别 | 行为 |
|------|------|
| `high` | 完整交互过渡、封面阴影与局部模糊 |
| `medium` | 平衡设置，移动端自动附加 `mobile-performance` 类 |
| `low` | 简化动画、降低模糊强度、移除封面阴影，歌词只保留简短透明度过渡 |
| `off` | 极简模式：禁用所有动画/模糊/阴影/渐变 |

### 移动端效果收敛

- 平衡 / 移动模式播放器模糊降到 8px，省电为 4px，极简完全移除。
- 不使用持续背景动画或 100px 背景光晕；封面轻阴影在省电模式移除。
- 极简与减少动态效果禁用动画及平滑滚动。歌词滚动必须读取实际性能偏好。
- 新增效果需覆盖 high / medium / low / off，不能只降级少量旧类名。
---

## PWA 配置

- `vite-plugin-pwa` 在 `vite.config.ts` 中配置
- Service Worker 策略：预缓存 + 运行时缓存 Google Fonts
- 更新检测：每 30 分钟检查一次（仅页面可见时）

---

## 环境变量

| 变量 | 说明 |
|------|------|
| `VITE_WS_URL` | WebSocket 服务器地址 |
| `VITE_API_URL` | REST API 基础地址 |
| `VITE_APP_NAME` | 应用显示名称 |

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
- **性能敏感**：新增动画/效果时，考虑在 `performance.css` 中添加对应级别的降级规则
- **移动端优先**：任何 UI 改动需同时考虑桌面端和移动端体验

---

*最后更新：2026年10月16日*
