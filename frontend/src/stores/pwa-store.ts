import { Store } from './store-base'

interface PWAState {
  showUpdateModal: boolean
  needRefresh: boolean
}

class PWAStore extends Store<PWAState> {
  private updateCheckTimer: ReturnType<typeof setInterval> | null = null
  private swRegistration: ServiceWorkerRegistration | undefined
  private updateServiceWorker: ((reloadPage?: boolean) => Promise<void>) | null = null
  private readonly CHECK_INTERVAL = 30 * 60 * 1000

  constructor() {
    super({ showUpdateModal: false, needRefresh: false })
    this.init()
  }

  private async init() {
    try {
      const { registerSW } = await import('virtual:pwa-register')
      this.updateServiceWorker = registerSW({
        onRegistered: (swRegistration) => {
          console.log('Service Worker 已注册')
          this.swRegistration = swRegistration
          this.startPeriodicUpdateCheck()
        },
        onRegisterError: (error: any) => {
          console.log('Service Worker 注册错误', error)
        },
        onNeedRefresh: () => {
          this.setState({ needRefresh: true, showUpdateModal: true })
        },
        onOfflineReady: () => {
          console.log('应用已准备好离线工作')
        },
      })
    } catch (e) {
      console.warn('PWA 注册失败:', e)
    }
  }

  private startPeriodicUpdateCheck() {
    if (this.updateCheckTimer)
      clearInterval(this.updateCheckTimer)
    this.updateCheckTimer = setInterval(async () => {
      try {
        if (document.visibilityState === 'visible' && this.swRegistration) {
          console.log('检查应用更新...')
          await this.swRegistration.update()
        }
      } catch (error) {
        console.error('检查更新失败:', error)
      }
    }, this.CHECK_INTERVAL)
    console.log(`已启动定时更新检查，间隔: ${this.CHECK_INTERVAL / 1000 / 60} 分钟`)
  }

  stopPeriodicUpdateCheck() {
    if (this.updateCheckTimer) {
      clearInterval(this.updateCheckTimer)
      this.updateCheckTimer = null
      console.log('已停止定时更新检查')
    }
  }

  handleUpdateApp() {
    if (this.updateServiceWorker) {
      this.updateServiceWorker(true)
    }
    this.setState({ showUpdateModal: false })
  }

  handleDismissUpdate() {
    this.setState({ showUpdateModal: false })
  }
}

export const pwaStore = new PWAStore()
