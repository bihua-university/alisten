export class Store<T extends Record<string, any>> extends EventTarget {
  private _state: T

  constructor(initialState: T) {
    super()
    this._state = { ...initialState }
  }

  get state(): T {
    return this._state
  }

  setState(partial: Partial<T>): void {
    const prev = { ...this._state }
    const next = { ...this._state, ...partial }
    const hasChanged = Object.keys(partial).some(key => prev[key] !== next[key])
    if (!hasChanged)
      return
    this._state = next
    this.dispatchEvent(new CustomEvent('change', { detail: { state: this._state, prev } }))
  }
}
