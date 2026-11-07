import { setColorScheme } from 'nsh'
import { localStorage } from '@/utils/storage'
import { uiFontSize } from '@/utils/ui-settings'
import { Store } from './store-base'

export type Theme = 'system' | 'light' | 'dark'

function loadAppearance(): { theme: Theme, fontSize: number } {
  try {
    const saved = JSON.parse(localStorage.getItem('alisten-appearance') || '{}')
    return {
      theme: ['system', 'light', 'dark'].includes(saved.theme) ? saved.theme : 'dark',
      fontSize: Math.max(12, Math.min(18, Number(saved.fontSize) || 14)),
    }
  } catch {
    return { theme: 'dark', fontSize: 14 }
  }
}

class AppearanceStore extends Store<{ theme: Theme, fontSize: number }> {
  constructor() {
    super(loadAppearance())
  }

  applySettings() {
    setColorScheme(this.state.theme)
    uiFontSize.set(this.state.fontSize)
  }

  update(theme: Theme, fontSize = this.state.fontSize) {
    this.setState({ theme, fontSize: Math.max(12, Math.min(18, fontSize)) })
    this.applySettings()
    try {
      localStorage.setItem('alisten-appearance', JSON.stringify(this.state))
    } catch (error) {
      console.warn('保存外观设置失败:', error)
    }
  }
}

export const appearanceStore = new AppearanceStore()
