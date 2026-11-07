import type { TextStyle, ViewStyle } from 'nsh'
import { colorScheme, reduceMotion, windowSize } from 'nsh'
import { performanceStore } from '@/stores/performance-store'
import { performanceRevision, uiFontSize } from '@/utils/ui-settings'

export const brand = '#68d268'
export const content = '#121212'
export const frame = '#000000'
export const foreground = '#f4f4f5'
export const muted = '#a6a6af'
export const subtle = '#90909a'
export const surface = '#232326'
export const surfaceHover = '#2c2c30'
export const card = '#181818'
export const border = '#333337'
export const input = '#29292d'
export const inputBorder = '#414148'
export const selected = '#3b3b3e'
export const accent = '#203320'
export const destructive = '#ff929a'
export const success = '#8cc9a5'
export const warning = '#eac17b'

export function palette(scheme: 'light' | 'dark' = colorScheme()) {
  if (scheme === 'light') {
    return {
      background: '#ffffff',
      foreground: '#202024',
      muted: '#64646e',
      subtle: '#71717b',
      surface: '#f3f3f6',
      surfaceHover: '#ededf0',
      popover: '#ffffff',
      input: '#f3f3f6',
      inputFocused: '#ffffff',
      border: '#e6e6eb',
      inputBorder: '#d4d4dc',
      selected: '#e4e4e8',
      accent: '#e7f6e7',
      toast: '#ffffff',
      destructive: '#b22638',
    }
  }
  return {
    background: '#1c1c1e',
    foreground,
    muted,
    subtle,
    surface,
    surfaceHover,
    popover: '#242427',
    input,
    inputFocused: '#303034',
    border,
    inputBorder,
    selected,
    accent,
    toast: '#303034',
    destructive,
  }
}

export function font(offset = 0): TextStyle {
  return { fontSize: uiFontSize() + offset }
}

export function text(offset = 0, color: string = foreground): TextStyle {
  return { fontSize: uiFontSize() + offset, color }
}

export function compact() {
  return windowSize().width <= 767
}

export function motionDuration(duration: number) {
  performanceRevision()
  if (reduceMotion() || performanceStore.state.level === 'off' || performanceStore.state.reducedMotion)
    return 0
  return performanceStore.state.level === 'low' ? Math.min(duration, 160) : duration
}

export const controlBase: ViewStyle = {
  minHeight: 36,
  paddingHorizontal: 14,
  paddingVertical: 8,
  borderRadius: 8,
  alignItems: 'center',
  justifyContent: 'center',
  cursor: 'pointer',
  transitionProperty: 'colors, opacity, transform',
  transitionDuration: 150,
}

export const focusRing: ViewStyle = {
  outlineWidth: 2,
  outlineColor: brand,
  outlineOffset: 3,
}
