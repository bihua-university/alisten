export interface AppConfig {
  websocket: {
    url: string
    reconnectAttempts: number
    reconnectInterval: number
    heartbeatInterval: number
  }
  api: {
    baseUrl: string
  }
  app: {
    name: string
    version: string
  }
}

const config: AppConfig = {
  websocket: {
    url: 'wss://bihua-alisten-api.ggemo.com',
    reconnectAttempts: 5,
    reconnectInterval: 3000,
    heartbeatInterval: 5000,
  },
  api: {
    baseUrl: 'https://bihua-alisten-api.ggemo.com',
  },
  app: {
    name: '同频',
    version: '0.0.1',
  },
}

export function getAppConfig(): AppConfig {
  return config
}

export function applyWebConfig(value: { VITE_WS_URL?: string, VITE_API_URL?: string, VITE_APP_NAME?: string }) {
  if (value.VITE_WS_URL)
    config.websocket.url = value.VITE_WS_URL
  if (value.VITE_API_URL)
    config.api.baseUrl = value.VITE_API_URL
  if (value.VITE_APP_NAME)
    config.app.name = value.VITE_APP_NAME
}

export function validateConfig(value: AppConfig): string[] {
  const errors: string[] = []
  if (!value.websocket.url)
    errors.push('WebSocket URL 未配置')
  if (!value.websocket.url.startsWith('ws://') && !value.websocket.url.startsWith('wss://'))
    errors.push('WebSocket URL 格式错误，必须以 ws:// 或 wss:// 开头')
  if (!value.api.baseUrl)
    errors.push('API 基础URL 未配置')
  return errors
}
