import { host, platform, Storage } from 'nsh'

const values = new Map<string, string>()

export async function hydrateStorage() {
  const keys = await Storage.keys()
  await Promise.all(keys.map(async (key) => {
    const value = await Storage.getItem(key)
    if (value !== null)
      values.set(key, value)
  }))
  if (platform.os === 'web' && host.available()) {
    const legacy = await host.call('legacyStorage.read') as Record<string, string | null>
    for (const [key, value] of Object.entries(legacy)) {
      if (value !== null)
        values.set(key, value)
    }
  }
}

export async function hydrateStorageKey(key: string) {
  if (!values.has(key)) {
    const value = await Storage.getItem(key)
    if (value !== null)
      values.set(key, value)
  }
  return values.get(key) ?? null
}

export const localStorage = {
  getItem(key: string) {
    return values.get(key) ?? null
  },
  setItem(key: string, value: string) {
    const text = String(value)
    values.set(key, text)
    void Storage.setItem(key, text).catch(error => console.warn('保存本地设置失败:', error))
    if (platform.os === 'web' && host.available())
      void host.call('legacyStorage.write', { key, value: text }).catch(error => console.warn('同步 Web 设置失败:', error))
  },
  removeItem(key: string) {
    values.delete(key)
    void Storage.removeItem(key).catch(error => console.warn('移除本地设置失败:', error))
    if (platform.os === 'web' && host.available())
      void host.call('legacyStorage.remove', { key }).catch(error => console.warn('同步 Web 设置失败:', error))
  },
}
