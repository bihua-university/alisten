import { createRoot } from 'react-dom/client'
import { App } from './app'
import { appearanceStore } from './stores/appearance-store'
import { performanceStore } from './stores/performance-store'
import './style.css'

performanceStore.applySettings()
appearanceStore.applySettings()
createRoot(document.getElementById('root')!).render(<App />)
