import type { PlayMode, User } from '@/types'
import { generateGravatarUrl } from '@/utils/user'
import { notificationStore } from './notification-store'
import { Store } from './store-base'
import { websocketStore } from './websocket-store'

function getStorageItem<T>(key: string, defaultValue: T): T {
  try {
    const item = localStorage.getItem(key)
    return item ? JSON.parse(item) : defaultValue
  } catch {
    return defaultValue
  }
}

function setStorageItem(key: string, value: any) {
  localStorage.setItem(key, JSON.stringify(value))
}

interface UserSettingsState {
  userName: string
  userEmail: string
  playMode: PlayMode
}

class UserSettingsStore extends Store<UserSettingsState> {
  constructor() {
    super({
      userName: getStorageItem('alisten_nickname', ''),
      userEmail: getStorageItem('alisten_email', ''),
      playMode: 'sequential',
    })

    websocketStore.registerEventHandler('settings.updated', (data) => {
      this.applyPlayMode(data.playMode)
    })
  }

  private applyPlayMode(mode: PlayMode) {
    if (mode === 'sequential' || mode === 'random')
      this.setState({ playMode: mode })
  }

  get currentUser(): User {
    const displayName = this.state.userName || '匿名用户'
    const avatar = this.state.userEmail
      ? generateGravatarUrl(this.state.userEmail, 200)
      : `https://api.dicebear.com/7.x/initials/svg?seed=${encodeURIComponent(displayName)}`
    return { name: displayName, avatar }
  }

  get emailValidation() {
    if (!this.state.userEmail)
      return { valid: true, message: '' }
    const valid = /^[\w.%+-]+@[\w.-]+\.[a-z]{2,}$/i.test(this.state.userEmail)
    return { valid, message: valid ? '' : '请输入有效的邮箱地址' }
  }

  setUserName(name: string) {
    this.setState({ userName: name })
    setStorageItem('alisten_nickname', name)
  }

  setUserEmail(email: string) {
    this.setState({ userEmail: email })
    setStorageItem('alisten_email', email)
  }

  async setPlayMode(mode: PlayMode) {
    try {
      const data = await websocketStore.request('settings.set', { playMode: mode })
      this.applyPlayMode(data.playMode)
      return true
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : '切换播放模式失败')
      return false
    }
  }

  async syncUserSettings(profile = { name: this.state.userName, email: this.state.userEmail }) {
    if (!profile.name)
      return true
    const clientTimeMs = Date.now()
    try {
      const res = await websocketStore.request('user.update', {
        name: profile.name,
        email: profile.email,
        clientTimeMs,
      })
      websocketStore.recordClockSample(clientTimeMs, Date.now(), res.serverTimeMs)
      return true
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : '同步用户信息失败')
      return false
    }
  }

  async pullSetting() {
    try {
      const data = await websocketStore.request('settings.get', {})
      this.applyPlayMode(data.playMode)
    } catch (error) {
      console.warn('拉取设置失败:', error instanceof Error ? error.message : error)
    }
  }
}

export const userSettingsStore = new UserSettingsStore()
