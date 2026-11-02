import { useCallback, useSyncExternalStore } from 'react'

interface ExternalStore<T> extends EventTarget {
  readonly state: T
}

/** Select stable fields/arrays so unrelated changes do not render this component. */
export function useStore<T, S>(store: ExternalStore<T>, selector: (state: T) => S): S {
  const subscribe = useCallback((notify: () => void) => {
    store.addEventListener('change', notify)
    return () => store.removeEventListener('change', notify)
  }, [store])
  return useSyncExternalStore(subscribe, () => selector(store.state))
}
