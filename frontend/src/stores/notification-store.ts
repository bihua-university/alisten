import { Store } from './store-base'

export interface NotificationOptions {
  message: string
  type?: 'success' | 'error' | 'warning' | 'info'
  icon?: string
  duration?: number
}

export interface NotificationItem extends NotificationOptions {
  id: string
  show: boolean
}

interface NotificationState {
  notifications: NotificationItem[]
}

let notificationId = 0

class NotificationStore extends Store<NotificationState> {
  constructor() {
    super({ notifications: [] })
  }

  show(options: NotificationOptions) {
    const id = `notification-${++notificationId}`
    const duration = options.duration ?? 3000
    const notification: NotificationItem = { id, show: true, ...options }
    this.setState({ notifications: [...this.state.notifications, notification] })
    setTimeout(() => this.hide(id), duration)
    return id
  }

  hide(id: string) {
    const notifications = this.state.notifications.map(n =>
      n.id === id ? { ...n, show: false } : n,
    )
    this.setState({ notifications })
    setTimeout(() => {
      this.setState({ notifications: this.state.notifications.filter(n => n.id !== id) })
    }, 300)
  }

  clear() {
    this.setState({ notifications: this.state.notifications.map(n => ({ ...n, show: false })) })
    setTimeout(() => this.setState({ notifications: [] }), 300)
  }

  success(message: string, options?: Omit<NotificationOptions, 'message' | 'type'>) {
    return this.show({ ...options, message, type: 'success', icon: options?.icon || 'fa-solid fa-check-circle' })
  }

  error(message: string, options?: Omit<NotificationOptions, 'message' | 'type'>) {
    return this.show({ ...options, message, type: 'error', icon: options?.icon || 'fa-solid fa-exclamation-circle' })
  }

  warning(message: string, options?: Omit<NotificationOptions, 'message' | 'type'>) {
    return this.show({ ...options, message, type: 'warning', icon: options?.icon || 'fa-solid fa-exclamation-triangle' })
  }

  info(message: string, options?: Omit<NotificationOptions, 'message' | 'type'>) {
    return this.show({ ...options, message, type: 'info', icon: options?.icon || 'fa-solid fa-info-circle' })
  }

  connectionSuccess() {
    return this.success('已连接到服务器', { icon: 'fa-solid fa-wifi' })
  }

  connectionError(message = '连接失败') {
    return this.error(message, { icon: 'fa-solid fa-wifi', duration: 5000 })
  }

  connectionWarning(message = '连接不稳定') {
    return this.warning(message, { icon: 'fa-solid fa-wifi' })
  }
}

export const notificationStore = new NotificationStore()
