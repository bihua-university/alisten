# Android 宿主

应用 ID 和 Kotlin 包名均为 `io.github.bihua_university.alisten`，源码目录为 `src/io/github/bihua_university/alisten/`，对应 GitHub 组织 `bihua-university/alisten`（包名中的连字符改为下划线）。NSH 的 AAR 仅作为 UI/JS 运行时依赖，通过公开的 `NshView` API 接入，不修改其源码或预编译运行时。

## 构建

需要 Python 3.9+、JDK 17+、Android SDK 35+、NSH CLI 和 Kotlin 编译器。配置 `ANDROID_HOME`、`JAVA_HOME`、`KOTLIN_HOME`，或将对应命令加入 PATH。`KOTLIN_HOME` 指向包含 `lib/kotlin-compiler.jar` 的目录；也可使用 Android Studio 的 `plugins/Kotlin/kotlinc`。

```powershell
npm run build:android
```

只构建 ARM64 压缩 Release，产物为 `.nsh/out/alisten-android-release/Alisten-<version>-arm64-v8a-compressed-release.apk`。脚本先用 NSH 生成临时基础包，再编译本目录 Kotlin 宿主、合并资源和额外 DEX、进行 16KB 对齐并签名验证。基础包和其他临时文件均位于 `.nsh/android-shell`，构建结束后自动删除，不会修改 NSH 安装目录。

使用 Android Studio 的 Kotlin 时自动采用同一套工具附带的 R8；独立 Kotlin 编译器需搭配兼容的 SDK D8，也可用 `R8_JAR` 指定。若 NSH 命令来自 shim，可用 `NSH_RUNTIME_HOME` 指定含 `embed/android/nsh.aar` 的运行时根目录。

默认使用与 NSH 相同的 `~/.android/debug.keystore`。发布构建可设置 `NSH_KEYSTORE_PATH`、`NSH_KEY_ALIAS`、`NSH_KEYSTORE_PASSWORD`；签名密码通过环境变量传给 apksigner，不写入代码和构建参数。

固定只打包 `arm64-v8a`，使用预编译 NSH release 运行时、D8 release 模式，并禁用应用调试。签名证书独立配置；没有配置正式证书时仍使用本地 debug key，便于覆盖开发安装，正式分发前需换用自己的发布证书。

原生库固定使用 ZIP 压缩，系统安装时解压到应用目录，以减小下载体积。也可直接运行 `python android/build.py`，不再提供架构、压缩方式或 Debug/Release 变体选项。

## 播放生命周期

- `AlistenActivity` 显示 NSH 界面，`PlaybackRuntime` 在播放服务存活时保留同一个引擎。Activity 重建后重新连接已有界面，避免重启房间和音频。
- 前台播放时通过窗口 `FLAG_KEEP_SCREEN_ON` 保持屏幕常亮；退到后台、停止播放或离开房间时清除标记，恢复系统息屏，不额外申请屏幕唤醒锁。
- `PlaybackService` 是 `mediaPlayback` 前台服务，显示持续媒体通知，持有局部 CPU 唤醒锁。旧系统另使用 Wi-Fi 性能锁；服务停止时释放资源。
- TS 层仅在连接、歌曲或播放状态变化时发布数据；媒体进度锚点最多每十秒校准一次。后台停止歌词动画、居中滚动及播放器视觉进度更新，保留音频、WebSocket 和重连。
- 服务每五秒发送原生连接检查事件，后台也能处理断线、退避重连及连接建立超时；默认网络重新可用或切换时立即重新连接，主动离房后不会重连。
- 服务统一管理音频焦点，NSH Video 不再重复申请焦点。被其他应用打断时暂停本地输出并阻止房间更新重新启动音频；焦点恢复后先请求最新播放状态，按服务器时间重新定位再续播。
- 永久丢失焦点不会收到系统 GAIN 回调，因此另监听系统播放活动：观察到播放停止后才尝试重新申请焦点；其他音频仍播放时不轮询抢焦点。设备必须能正常报告播放活动，系统强制停止应用无法自动恢复。
- 原生 `MediaSession` 发布标题、歌手、专辑、封面、时长、播放进度和状态。通知和外部媒体控制提供投票切歌、停止及返回应用；不向房间添加暂停或 seek 操作。
- 通知中的停止操作会断开房间、停止音频、移除通知并释放锁。服务不在系统或用户停止后自行复活，不注册开机广播、不播放无声音频。

前台服务支持退到桌面、锁屏和 Activity 重建后的持续会话；系统强制停止应用仍会结束播放。厂商额外的后台限制由系统设置控制。第三方灵动岛组件需自行支持 Android 媒体会话，应用只发布标准媒体信息。
