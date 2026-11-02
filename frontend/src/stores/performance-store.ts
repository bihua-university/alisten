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
    if (typeof window !== 'undefined') {
      this.loadSettings()
      this.applySettings()
      if (!localStorage.getItem('alisten-performance-level')) {
        this.autoDetect()
      }
    }
  }

  private loadSettings() {
    try {
      const saved = localStorage.getItem('alisten-performance-level')
      if (saved && ['high', 'medium', 'low', 'off'].includes(saved)) {
        this.setState({ level: saved as PerformanceLevel })
      }

      const motionSaved = localStorage.getItem('alisten-reduced-motion')
      const motionSource = localStorage.getItem('alisten-reduced-motion-source') as MotionPreferenceSource | null

      if (motionSaved && motionSource === 'manual') {
        this.setState({ reducedMotion: JSON.parse(motionSaved) })
      } else if (typeof window !== 'undefined') {
        const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
        this.setState({ reducedMotion: prefersReducedMotion })
        this.saveSettings('system')
      }
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
    const body = document.body
    body.classList.remove('performance-high', 'performance-medium', 'performance-low', 'performance-off', 'no-animations', 'mobile-performance')

    switch (this.state.level) {
      case 'high':
        body.classList.add('performance-high')
        break
      case 'medium':
        body.classList.add('performance-medium')
        if (window.innerWidth <= 768)
          body.classList.add('mobile-performance')
        break
      case 'low':
        body.classList.add('performance-low', 'mobile-performance')
        break
      case 'off':
        body.classList.add('performance-off', 'no-animations')
        break
    }

    if (this.state.reducedMotion) {
      body.classList.add('no-animations')
    }
  }

  autoDetect() {
    const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    this.setState({ reducedMotion: prefersReducedMotion })

    const isMobile = window.innerWidth <= 768
    const isLowEnd = (navigator as any).deviceMemory && (navigator as any).deviceMemory <= 4
    if (isMobile || isLowEnd) {
      this.setState({ level: 'low' })
    } else if ((navigator as any).deviceMemory && (navigator as any).deviceMemory >= 8) {
      this.setState({ level: 'high' })
    } else {
      this.setState({ level: 'medium' })
    }
    this.applySettings()
    this.saveSettings('system')
  }

  getPerformanceClasses(): string[] {
    const classes = [`performance-${this.state.level}`]
    if (this.state.reducedMotion)
      classes.push('no-animations')
    if (typeof window !== 'undefined' && window.innerWidth <= 768) {
      classes.push('mobile-performance')
    }
    return classes
  }

  getDescription(level = this.state.level): string {
    switch (level) {
      case 'high': return '高质量 - 所有动画效果，适合高性能设备'
      case 'medium': return '平衡 - 优化过的动画效果，推荐设置'
      case 'low': return '省电 - 简化动画效果，适合移动设备'
      case 'off': return '极简 - 禁用所有动画，最低GPU占用'
      default: return '未知设置'
    }
  }

  setHigh() {
    this.setState({ level: 'high' })
    this.applySettings()
    this.saveSettings()
  }

  setMedium() {
    this.setState({ level: 'medium' })
    this.applySettings()
    this.saveSettings()
  }

  setLow() {
    this.setState({ level: 'low' })
    this.applySettings()
    this.saveSettings()
  }

  setOff() {
    this.setState({ level: 'off' })
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
