import type { Signal } from 'nsh'
import type { Store } from '@/stores/store-base'
import { cleanup, signal } from 'nsh'

export function useStore<T extends Record<string, any>, S>(store: Store<T>, selector: (state: T) => S): Signal<S> {
  const value = signal(selector(store.state))
  const update = () => value.set(selector(store.state))
  store.addEventListener('change', update)
  cleanup(() => store.removeEventListener('change', update))
  return value
}
