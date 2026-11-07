import { platform, reduceMotion } from 'nsh'
import { localStorage } from '@/utils/storage'
import { performanceRevision } from '@/utils/ui-settings'
import { Store } from './store-base'

export type PerformanceLevel = 'high' | 'medium' | 'low' | 'off'

interface PerformanceState {
  level: PerformanceLevel
  reducedMotion: boolean
}

type MotionPreferenceSource = 'system' | 'manual'

class PerformanceStore extends Store<PerformanceState> {
  constructor() {
    super({ level: 'medium', reducedMotion: false })
    this.loadSettings()
    if (!localStorage.getItem('alisten-performance-level'))
      this.autoDetect()
    this.applySettings()
  }

  private loadSettings() {
    try {
      const saved = localStorage.getItem('alisten-performance-level')
      if (saved && ['high', 'medium', 'low', 'off'].includes(saved))
        this.setState({ level: saved as PerformanceLevel })
      const motionSaved = localStorage.getItem('alisten-reduced-motion')
      const motionSource = localStorage.getItem('alisten-reduced-motion-source') as MotionPreferenceSource | null
      if (motionSaved && motionSource === 'manual')
        this.setState({ reducedMotion: JSON.parse(motionSaved) })
      else
        this.setState({ reducedMotion: reduceMotion() })
    } catch (error) {
      console.warn('读取性能设置失败:', error)
    }
  }

  private getMotionPreferenceSource(): MotionPreferenceSource {
    return localStorage.getItem('alisten-reduced-motion-source') === 'manual' ? 'manual' : 'system'
  }

  saveSettings(motionSource = this.getMotionPreferenceSource()) {
    try {
      localStorage.setItem('alisten-performance-level', this.state.level)
      localStorage.setItem('alisten-reduced-motion', JSON.stringify(this.state.reducedMotion))
      localStorage.setItem('alisten-reduced-motion-source', motionSource)
    } catch (error) {
      console.warn('保存性能设置失败:', error)
    }
  }

  applySettings() {
    performanceRevision.update(value => value + 1)
  }

  autoDetect() {
    this.setState({
      reducedMotion: reduceMotion(),
      level: platform.desktop || platform.os === 'android' ? 'medium' : 'low',
    })
    this.applySettings()
    this.saveSettings('system')
  }

  getPerformanceClasses(): string[] {
    return [`performance-${this.state.level}`, ...this.state.reducedMotion ? ['no-animations'] : []]
  }

  getDescription(level = this.state.level): string {
    switch (level) {
      case 'high': return '高质量 - 所有动画效果，适合高性能设备'
      case 'medium': return '平衡 - 优化过的动画效果，推荐设置'
      case 'low': return '省电 - 简化动画效果，适合移动设备'
      case 'off': return '极简 - 禁用所有动画，最低GPU占用'
    }
  }

  setHigh() {
    this.setLevel('high')
  }

  setMedium() {
    this.setLevel('medium')
  }

  setLow() {
    this.setLevel('low')
  }

  setOff() {
    this.setLevel('off')
  }

  private setLevel(level: PerformanceLevel) {
    this.setState({ level })
    this.applySettings()
    this.saveSettings()
  }

  toggleReducedMotion() {
    this.setState({ reducedMotion: !this.state.reducedMotion })
    this.applySettings()
    this.saveSettings('manual')
  }
}

export const performanceStore = new PerformanceStore()
