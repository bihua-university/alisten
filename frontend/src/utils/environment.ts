import { getLocation, navigateApp } from './runtime'
import { localStorage } from './storage'

export type Environment = 'prod' | 'canary'

export function getEnvironment(): Environment {
  return getLocation().searchParams.get('env') === 'canary' ? 'canary' : 'prod'
}

export async function setEnvironment(next: Environment) {
  if (next !== 'prod' && next !== 'canary')
    throw new Error('未知环境')
  if (next === getEnvironment())
    return
  const { websocketStore } = await import('@/stores/websocket-store')
  websocketStore.disconnect()
  localStorage.removeItem('alisten_last_joined_room')
  localStorage.removeItem('alisten_last_joined_room-canary')
  const url = getLocation()
  for (const key of ['houseId', 'houseid', 'HOUSEID', 'housePwd', 'housepwd', 'HOUSEPWD'])
    url.searchParams.delete(key)
  if (next === 'canary')
    url.searchParams.set('env', 'canary')
  else
    url.searchParams.delete('env')
  await navigateApp(url)
}

export function applyEnvironment(url: URL): URL {
  if (getEnvironment() === 'canary')
    url.searchParams.set('env', 'canary')
  else
    url.searchParams.delete('env')
  return url
}

export function environmentStorageKey(key: string): string {
  return getEnvironment() === 'canary' ? `${key}-canary` : key
}
