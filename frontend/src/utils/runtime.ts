import { Clipboard, host, platform } from 'nsh'
import { applyWebConfig } from './config'
import { hydrateLifecycle } from './lifecycle'

let currentLocation = new URL('https://alisten.local/')
let browserPlayback = false

export async function hydrateRuntime() {
  if (platform.os === 'web' && host.available()) {
    const result = await host.call('app.location') as { href: string, config?: Parameters<typeof applyWebConfig>[0], mediaPlayback?: boolean }
    if (result.config)
      applyWebConfig(result.config)
    browserPlayback = result.mediaPlayback === true
    currentLocation = new URL(result.href)
    await hydrateLifecycle()
  }
}

export function supportsBrowserPlayback() {
  return browserPlayback
}

export function getLocation() {
  return new URL(currentLocation.href)
}

export function setDocumentTitle(title: string) {
  if (platform.os === 'web' && host.available())
    void host.call('app.title', { title }).catch(error => console.warn('更新标题失败:', error))
}

export function reloadApp() {
  if (platform.os === 'web' && host.available())
    void host.call('app.reload')
}

export async function navigateApp(url: URL) {
  if (platform.os === 'web' && host.available())
    await host.call('app.navigate', { url: url.href })
  else
    currentLocation = url
}

export async function shareRoom(title: string, url: string) {
  if (platform.os === 'web' && host.available())
    return await host.call('app.share', { title, text: '来和我一起听歌吧！', url }) as string
  await Clipboard.writeText(url)
  return 'copied'
}

export async function downloadText(filename: string, contents: string, type: string) {
  if (platform.os === 'web' && host.available())
    await host.call('app.download', { filename, contents, type })
}

export function onHostEvent(event: string, listener: (payload: any) => void) {
  if (!host.available())
    return () => {}
  return host.on(event, listener)
}
