import { createRoot } from 'react-dom/client'
import { App } from './app'
import { appearanceStore } from './stores/appearance-store'
import { performanceStore } from './stores/performance-store'
import { getEnvironment, setEnvironment } from './utils/environment'
import './style.css'

Object.assign(window, { alistenEnvironment: { getEnvironment, setEnvironment } })
if (getEnvironment() === 'canary')
  document.title += ' · Canary'

performanceStore.applySettings()
appearanceStore.applySettings()
createRoot(document.getElementById('root')!).render(<App />)
