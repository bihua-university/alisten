export async function createHost(player) {
  const playAudio = async ({ source }) => {
    const expected = new URL(source, location.href).href
    const audio = document.querySelector('[data-testid="alisten-audio"] video')
    if (!audio || audio.src !== expected)
      return { status: 'aborted' }
    try {
      await audio.play()
      return { status: audio.isConnected && audio.src === expected ? 'playing' : 'aborted' }
    } catch (error) {
      if (!audio.isConnected || audio.src !== expected || error.name === 'AbortError')
        return { status: 'aborted' }
      return { status: error.name === 'NotAllowedError' ? 'blocked' : 'error', message: error.message }
    }
  }
  const functions = new Map([
    ['app.location', () => ({ href: location.href, config: globalThis.__ALISTEN_CONFIG__ ?? {}, mediaPlayback: true })],
    ['app.lifecycle', () => ({ state: document.hidden ? 'background' : 'active' })],
    ['app.title', ({ title }) => {
      document.title = String(title)
    }],
    ['app.reload', () => location.reload()],
    ['app.navigate', ({ url }) => location.replace(String(url))],
    ['app.open', ({ url }) => {
      window.open(String(url), '_blank', 'noopener,noreferrer')
    }],
    ['app.share', async ({ title, text, url }) => {
      if (navigator.share) {
        await navigator.share({ title, text, url })
        return 'shared'
      }
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(String(url))
        return 'copied'
      }
      window.prompt('复制房间链接', String(url))
      return 'prompted'
    }],
    ['app.download', ({ contents, filename, type }) => {
      const blob = new Blob([String(contents)], { type: String(type) })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = String(filename)
      link.click()
      setTimeout(() => URL.revokeObjectURL(url), 0)
    }],
    ['legacyStorage.read', () => Object.fromEntries(Array.from({ length: localStorage.length }, (_, index) => {
      const key = localStorage.key(index)
      return [key, key === null ? null : localStorage.getItem(key)]
    }).filter(([key]) => key !== null))],
    ['legacyStorage.write', ({ key, value }) => {
      localStorage.setItem(String(key), String(value))
    }],
    ['legacyStorage.remove', ({ key }) => {
      localStorage.removeItem(String(key))
    }],
    ['media.play', playAudio],
    ['media.update', ({ song }) => {
      if (!('mediaSession' in navigator))
        return false
      navigator.mediaSession.metadata = song ? new MediaMetadata(song) : null
      navigator.mediaSession.playbackState = song ? 'playing' : 'none'
      return true
    }],
    ['media.actions', ({ actions }) => {
      if (!('mediaSession' in navigator))
        return false
      for (const action of ['play', 'pause', 'previoustrack', 'nexttrack', 'seekbackward', 'seekforward', 'stop']) {
        try {
          navigator.mediaSession.setActionHandler(action, actions.includes(action) ? () => player.hostEvent('media.action', { action }) : null)
        } catch {}
      }
      return true
    }],
  ])

  if (typeof addEventListener === 'function') {
    addEventListener('online', () => player.hostEvent('app.online', {}))
    addEventListener('offline', () => player.hostEvent('app.offline', {}))
    addEventListener('pageshow', event => player.hostEvent('app.pageshow', { persisted: event.persisted }))
    document.addEventListener('visibilitychange', () => player.hostEvent('app.lifecycle', { state: document.hidden ? 'background' : 'active' }))
  }
  return {
    call(name, args) {
      const fn = functions.get(name)
      if (!fn)
        throw new Error(`no host function ${name}`)
      return fn(args ?? {})
    },
  }
}
