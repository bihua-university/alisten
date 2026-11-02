import type { ConnectionStatus, WebSocketConfig } from '@/types'
import type {
  ErrorCode,
  EventDataMap,
  EventType,
  RequestDataMap,
  RequestType,
  ResponseDataMap,
  Source,
} from '@/types/ws-protocol'
import { notificationStore } from './notification-store'
import { roomStore } from './room-store'
import { Store } from './store-base'

interface WebSocketState {
  ws: WebSocket | null
  connectionStatus: ConnectionStatus
  isConnecting: boolean
  reconnectAttempts: number
}

const config: WebSocketConfig = {
  url: import.meta.env.VITE_WS_URL || 'ws://localhost:8080',
  reconnectAttempts: 5,
  reconnectInterval: 3000,
  heartbeatInterval: 5000,
}

/**
 * 重连退避上限。重连不再设次数上限：
 * 移动端切后台后连接会被系统回收，原先「最多 5 次、间隔 3 秒」的策略会在 15 秒后
 * 永久放弃，回到前台也不会自愈，必须手动刷新页面。
 */
const MAX_RECONNECT_INTERVAL = 30_000

/** 普通请求超时；服务端单次请求处理上下文最长 3 分钟，超时后不自动重放 */
const REQUEST_TIMEOUT = 60_000
/** 心跳超时仅用于清理 pending 项，不作为连接死活依据 */
const HEARTBEAT_TIMEOUT = 10_000

export class RequestError extends Error {
  code: ErrorCode | 'CLIENT_ERROR'
  constructor(code: ErrorCode | 'CLIENT_ERROR', message: string) {
    super(message)
    this.code = code
  }
}

interface PendingRequest {
  type: RequestType
  resolve: (data: any) => void
  reject: (error: RequestError) => void
  timeoutTimer: ReturnType<typeof setTimeout>
}

type EventHandler<T extends EventType = EventType> = (data: EventDataMap[T]) => void

const pendingRequests = new Map<string, PendingRequest>()
const eventHandlers = new Map<EventType, EventHandler<any>>()

let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let heartbeatTimer: ReturnType<typeof setInterval> | null = null

/** 用户主动断开标记：为真时 onclose 不再触发重连 */
let manuallyClosed = false

let lifecycleBound = false

/** 时钟样本：优先使用往返时延最小的样本估计时钟偏移 */
let clockOffsetMs = 0
let bestRoundTripMs = Number.POSITIVE_INFINITY

class WebSocketStore extends Store<WebSocketState> {
  constructor() {
    super({ ws: null, connectionStatus: 'disconnected', isConnecting: false, reconnectAttempts: 0 })
  }

  /**
   * 注册广播事件处理器。事件与请求响应分离路由，
   * 同一个 type 的事件与响应不会互相触发。
   */
  registerEventHandler<T extends EventType>(type: T, handler: EventHandler<T>) {
    eventHandlers.set(type, handler)
  }

  getClockOffsetMs(): number {
    return clockOffsetMs
  }

  /**
   * 由心跳或 user.update 响应带来的时钟样本：
   * offset = S - (C0 + C1) / 2，取往返时延最小的样本。
   */
  recordClockSample(c0: number, c1: number, serverTimeMs: number) {
    const roundTripMs = c1 - c0
    if (roundTripMs > bestRoundTripMs)
      return
    bestRoundTripMs = roundTripMs
    clockOffsetMs = serverTimeMs - (c0 + c1) / 2
  }

  private clearPendingRequests(reason: string) {
    pendingRequests.forEach((pending) => {
      clearTimeout(pending.timeoutTimer)
      pending.reject(new RequestError('CLIENT_ERROR', reason))
    })
    pendingRequests.clear()
  }

  private handleMessage(event: MessageEvent) {
    let message: any
    try {
      if (!event.data)
        return
      message = JSON.parse(event.data)
    } catch {
      console.warn('收到非 JSON 消息:', event.data)
      return
    }
    if (!message || typeof message !== 'object' || message.version !== 1 || typeof message.type !== 'string') {
      console.warn('收到无效的消息格式:', event.data)
      return
    }

    if (message.kind === 'response') {
      this.handleResponse(message)
      return
    }
    if (message.kind === 'event') {
      if (message.type === 'protocol.error')
        notificationStore.error(message.data?.message || '协议错误')
      eventHandlers.get(message.type as EventType)?.(message.data)
      return
    }
    console.warn('收到未知方向的消息:', event.data)
  }

  private handleResponse(message: { type: string, requestId?: string, data?: any, error?: { code: ErrorCode, message: string } }) {
    const requestId = message.requestId
    if (!requestId) {
      console.warn('响应缺少 requestId:', message)
      return
    }
    const pending = pendingRequests.get(requestId)
    if (!pending) {
      console.warn('收到未知请求的响应:', message.type, requestId)
      return
    }
    pendingRequests.delete(requestId)
    clearTimeout(pending.timeoutTimer)
    if (pending.type !== message.type) {
      pending.reject(new RequestError('CLIENT_ERROR', `响应类型不匹配: 期望 ${pending.type}，收到 ${message.type}`))
      return
    }
    if (message.error) {
      pending.reject(new RequestError(message.error.code, message.error.message))
      return
    }
    pending.resolve(message.data)
  }

  private clearReconnectTimer() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
  }

  private handleConnectionLoss(ws: WebSocket) {
    if (this.state.ws !== ws)
      return

    ws.onopen = null
    ws.onmessage = null
    ws.onclose = null
    ws.onerror = null
    this.stopHeartbeat()
    this.clearPendingRequests('连接已断开')
    this.resetClockSamples()
    this.setState({ ws: null, connectionStatus: 'disconnected', isConnecting: false })
    if (ws.readyState !== WebSocket.CLOSED) {
      try {
        ws.close()
      } catch (error) {
        console.warn('关闭失效连接失败:', error)
      }
    }
    if (!manuallyClosed)
      this.scheduleReconnect()
  }

  connect() {
    if (this.state.isConnecting || this.state.connectionStatus === 'connected')
      return

    const password = roomStore.getCurrentPassword()
    if (roomStore.state.needPwd && !password) {
      console.warn('房间需要密码但未提供密码')
      return
    }

    this.clearReconnectTimer()
    manuallyClosed = false
    this.setState({ isConnecting: true, connectionStatus: 'connecting' })
    this.bindLifecycle()

    try {
      const params = new URLSearchParams({ houseId: roomStore.state.id })
      if (password)
        params.set('housePwd', password)
      const wsUrl = `${config.url}/server?${params.toString()}`

      console.log('连接 WebSocket:', wsUrl)
      const ws = new WebSocket(wsUrl)

      ws.onopen = () => {
        if (this.state.ws !== ws || manuallyClosed)
          return
        console.log('WebSocket 连接已建立')
        this.setState({ connectionStatus: 'connected', isConnecting: false, reconnectAttempts: 0 })
        if (this.state.ws === ws && !manuallyClosed)
          this.startHeartbeat()
      }

      ws.onmessage = (event) => {
        if (this.state.ws === ws && !manuallyClosed)
          this.handleMessage(event)
      }

      ws.onclose = (event) => {
        console.log('WebSocket 连接已关闭:', event.code, event.reason)
        this.handleConnectionLoss(ws)
      }

      ws.onerror = (error) => {
        console.error('WebSocket 连接错误:', error)
        this.handleConnectionLoss(ws)
      }

      this.setState({ ws })
    } catch (error) {
      console.error('创建 WebSocket 连接失败:', error)
      this.setState({ connectionStatus: 'error', isConnecting: false })
      if (!manuallyClosed)
        this.scheduleReconnect()
    }
  }

  private resetClockSamples() {
    clockOffsetMs = 0
    bestRoundTripMs = Number.POSITIVE_INFINITY
  }

  /**
   * 指数退避重连：3s → 6s → 12s → 24s → 30s(封顶)，不设次数上限。
   * 上限只影响重试频率，不会再出现「重试耗尽后永久失联」。
   */
  private scheduleReconnect() {
    if (manuallyClosed || reconnectTimer)
      return

    const attempts = this.state.reconnectAttempts + 1
    const delay = Math.min(
      config.reconnectInterval * 2 ** (attempts - 1),
      MAX_RECONNECT_INTERVAL,
    )

    this.setState({ connectionStatus: 'reconnecting', reconnectAttempts: attempts })
    console.log(`准备第 ${attempts} 次重连，${Math.round(delay / 1000)}s 后重试...`)

    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      this.connect()
    }, delay)
  }

  /**
   * 心跳：每 5 秒发送一次并携带本地时间，服务器响应用于估计时钟偏移。
   *
   * 不做 visibility 判断——桌面端后台标签页仍需保持连接以维持播放，
   * 浏览器自身会节流后台定时器；移动端页面被冻结时定时器停止，
   * 由回前台的 readyState 检查兜底。
   *
   * 注意：不能仅凭「未收到心跳响应」判断断线（静音房间和回前台会被误判），
   * 连接失效依赖不依赖回包的信号：死连接上的 write 触发 onerror/onclose，
   * 回前台时检查 readyState。
   */
  private startHeartbeat() {
    this.stopHeartbeat()
    heartbeatTimer = setInterval(() => {
      this.sendHeartbeat()
    }, config.heartbeatInterval)
  }

  private async sendHeartbeat() {
    if (!this.state.ws || this.state.ws.readyState !== WebSocket.OPEN)
      return
    const c0 = Date.now()
    try {
      const res = await this.request('heartbeat', { clientTimeMs: c0 }, HEARTBEAT_TIMEOUT)
      this.recordClockSample(c0, Date.now(), res.serverTimeMs)
    } catch {
      // 心跳失败仅清理 pending，不触发重连
    }
  }

  private stopHeartbeat() {
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer)
      heartbeatTimer = null
    }
  }

  /**
   * 立即重连：强制丢弃当前 socket 并重置退避计数。
   * 用于回前台、网络恢复、以及已确认失效的连接。
   */
  private restartConnection(reason: string) {
    this.clearReconnectTimer()
    this.stopHeartbeat()
    this.clearPendingRequests('连接已断开')
    this.resetClockSamples()

    const ws = this.state.ws
    // 先置空再关闭：摘掉回调后 onclose 不会再调度重连，避免重复重连
    this.setState({ ws: null, isConnecting: false, reconnectAttempts: 0, connectionStatus: 'disconnected' })

    if (ws) {
      ws.onopen = null
      ws.onmessage = null
      ws.onclose = null
      ws.onerror = null
      try {
        ws.close(1000, reason)
      } catch (error) {
        console.warn('关闭旧连接失败:', error)
      }
    }

    this.connect()
  }

  /**
   * 回前台时的连接检查。
   *
   * 切后台再回来时 socket 常仍报 OPEN 但已被系统回收，而不假定服务端回应心跳，
   * 就无法靠回包确证存活。因此这里分两种情况处理：
   * - 非 OPEN：直接重连
   * - OPEN：立刻补一次心跳，死连接上的写入会触发 onclose/onerror，交由正常重连路径接管
   */
  private verifyConnectionOnResume(reason: string) {
    const ws = this.state.ws
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      this.restartConnection(reason)
      return
    }

    this.sendHeartbeat()
  }

  private handleVisibilityChange = () => {
    if (document.hidden || manuallyClosed)
      return

    if (this.state.connectionStatus === 'connected') {
      this.verifyConnectionOnResume('回到前台')
      return
    }

    // 已有连接正在建立时不必打断握手
    if (this.state.isConnecting)
      return

    console.log('回到前台，立即重连')
    this.restartConnection('回到前台')
  }

  private handleOnline = () => {
    if (manuallyClosed || this.state.isConnecting)
      return
    if (this.state.connectionStatus === 'connected') {
      this.verifyConnectionOnResume('网络恢复')
      return
    }
    console.log('网络已恢复，立即重连')
    this.restartConnection('网络恢复')
  }

  /** bfcache 恢复：整个页面被冻结后唤醒，socket 基本已经失效 */
  private handlePageShow = (event: PageTransitionEvent) => {
    if (!event.persisted || manuallyClosed)
      return
    this.restartConnection('页面从缓存恢复')
  }

  private bindLifecycle() {
    if (lifecycleBound || typeof document === 'undefined')
      return
    lifecycleBound = true
    document.addEventListener('visibilitychange', this.handleVisibilityChange)
    window.addEventListener('online', this.handleOnline)
    window.addEventListener('pageshow', this.handlePageShow)
  }

  private unbindLifecycle() {
    if (!lifecycleBound)
      return
    lifecycleBound = false
    document.removeEventListener('visibilitychange', this.handleVisibilityChange)
    window.removeEventListener('online', this.handleOnline)
    window.removeEventListener('pageshow', this.handlePageShow)
  }

  disconnect() {
    manuallyClosed = true
    this.clearReconnectTimer()
    this.stopHeartbeat()
    this.unbindLifecycle()
    this.clearPendingRequests('连接已断开')
    this.resetClockSamples()

    const ws = this.state.ws
    this.setState({ ws: null, connectionStatus: 'disconnected', isConnecting: false, reconnectAttempts: 0 })

    if (ws) {
      ws.onopen = null
      ws.onmessage = null
      ws.onclose = null
      ws.onerror = null
      try {
        ws.close(1000, '用户主动断开连接')
      } catch (error) {
        console.warn('关闭连接失败:', error)
      }
    }

    roomStore.clearCurrentPassword()
  }

  reconnect() {
    this.restartConnection('手动重连')
  }

  /**
   * 发送协议请求并等待关联响应。先登记 pending 再写入 socket；
   * 超时或断线会 reject，不会自动重放写操作。
   */
  request<T extends RequestType>(
    type: T,
    data: RequestDataMap[T],
    timeoutMs: number = REQUEST_TIMEOUT,
  ): Promise<ResponseDataMap[T]> {
    const ws = this.state.ws
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      if (ws && (ws.readyState === WebSocket.CLOSED || ws.readyState === WebSocket.CLOSING))
        this.handleConnectionLoss(ws)
      console.warn('WebSocket 未连接，请求发送失败:', type)
      return Promise.reject(new RequestError('CLIENT_ERROR', '与服务器断开连接'))
    }

    const requestId = crypto.randomUUID()
    return new Promise<ResponseDataMap[T]>((resolve, reject) => {
      const timeoutTimer = setTimeout(() => {
        pendingRequests.delete(requestId)
        reject(new RequestError('CLIENT_ERROR', '请求超时'))
      }, timeoutMs)
      pendingRequests.set(requestId, { type, resolve, reject, timeoutTimer })
      try {
        ws.send(JSON.stringify({ version: 1, kind: 'request', type, requestId, data }))
      } catch (error) {
        pendingRequests.delete(requestId)
        clearTimeout(timeoutTimer)
        reject(new RequestError('CLIENT_ERROR', `消息写入失败: ${error}`))
        this.handleConnectionLoss(ws)
      }
    })
  }

  sendChatMessage(content: string) {
    if (!content || typeof content !== 'string')
      return false
    const trimmed = content.trim()
    if (!trimmed || trimmed.length > 500) {
      console.warn('消息长度不能超过500个字符')
      return false
    }
    // Try command handling
    if (this.handleCommand(trimmed))
      return true
    this.sendChat(trimmed)
    return true
  }

  private async sendChat(content: string) {
    try {
      await this.request('chat.send', { content })
    } catch (error) {
      notificationStore.error((error as RequestError).message)
    }
  }

  private handleCommand(content: string): boolean {
    const trimmed = content.trim()
    if (trimmed.startsWith('点歌')) {
      const args = trimmed.substring(2).trim()
      if (!args || !roomStore.state.ultimate)
        return false
      const isUrl = args.startsWith('http://') || args.startsWith('https://')
      const source: Source = isUrl ? 'url_common' : 'netease'
      this.pickByCommand(source, isUrl ? args : undefined, isUrl ? undefined : args)
      return true
    }
    return false
  }

  private async pickByCommand(source: Source, id?: string, name?: string) {
    try {
      const res = await this.request('music.pick', { source, id, name } as RequestDataMap['music.pick'])
      notificationStore.success(`已发送点歌请求: ${res.name}`)
    } catch (error) {
      notificationStore.error((error as RequestError).message)
    }
  }

  async sendSongLike(id: string, source: Source) {
    try {
      await this.request('queue.like', { id, source })
      notificationStore.success('已点赞歌曲')
    } catch (error) {
      notificationStore.error((error as RequestError).message)
    }
  }

  async sendDeleteSong(id: string, source: Source) {
    try {
      await this.request('queue.remove', { id, source })
      notificationStore.success('已移除歌曲')
    } catch (error) {
      notificationStore.error((error as RequestError).message)
    }
  }

  getConfig() {
    return config
  }
}

export const websocketStore = new WebSocketStore()
