import { host, lifecycle as nativeLifecycle, platform, signal } from 'nsh'

const state = signal<'active' | 'inactive' | 'background'>('active')

export function lifecycle() {
  return platform.os === 'web' ? state() : nativeLifecycle()
}

export async function hydrateLifecycle() {
  if (platform.os !== 'web' || !host.available())
    return
  const current = await host.call('app.lifecycle') as { state: 'active' | 'inactive' | 'background' }
  state.set(current.state)
  host.on('app.lifecycle', ({ state: next }: { state: 'active' | 'inactive' | 'background' }) => {
    state.set(next)
  })
}
