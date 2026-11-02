import type { ReactNode } from 'react'
import type { Theme } from '@/stores/appearance-store'
import type { PerformanceLevel } from '@/stores/performance-store'
import type { PlayMode } from '@/types'
import { useEffect, useRef, useState } from 'react'
import { useStore } from '@/hooks/use-store'
import { appearanceStore } from '@/stores/appearance-store'
import { historyStore } from '@/stores/history-store'
import { notificationStore } from '@/stores/notification-store'
import { performanceStore } from '@/stores/performance-store'
import { playerStore } from '@/stores/player-store'
import { userSettingsStore } from '@/stores/user-settings-store'
import { websocketStore } from '@/stores/websocket-store'
import { getAppConfig } from '@/utils/config'

const performanceActions = {
  high: () => performanceStore.setHigh(),
  medium: () => performanceStore.setMedium(),
  low: () => performanceStore.setLow(),
  off: () => performanceStore.setOff(),
}

interface SettingsDraft {
  name: string
  email: string
  theme: Theme
  fontSize: number
  volume: number
  muted: boolean
  mode: PlayMode
  level: PerformanceLevel
  reducedMotion: boolean
  enableSearchHistory: boolean
  historyCount: number
  isRecording: boolean
  token: string
  cookie: string
}

export default function SettingsPage() {
  const heading = useRef<HTMLHeadingElement>(null)
  const unlockClicks = useRef({ count: 0, lastClickAt: 0 })
  const [cookieUnlocked, setCookieUnlocked] = useState(false)
  useEffect(() => heading.current?.focus({ preventScroll: true }), [])
  const profile = useStore(userSettingsStore, s => s)
  const performance = useStore(performanceStore, s => s)
  const appearance = useStore(appearanceStore, s => s)
  const volume = useStore(playerStore, s => s.volume)
  const muted = useStore(playerStore, s => s.isMuted)
  const enableSearchHistory = useStore(historyStore, s => s.enableSearchHistory)
  const historyCount = useStore(historyStore, s => s.maxSearchHistoryCount)
  const isRecording = useStore(historyStore, s => s.isRecording)
  const [savedToken, setSavedToken] = useState(() => localStorage.getItem('alisten-cookie-admin-token') || '')
  const [changes, setChanges] = useState<Partial<SettingsDraft>>({})
  const [saving, setSaving] = useState(false)
  const savingRef = useRef(false)
  const [error, setError] = useState('')
  const [cookieStatus, setCookieStatus] = useState('未查询')
  const current: SettingsDraft = {
    name: profile.userName,
    email: profile.userEmail,
    theme: appearance.theme,
    fontSize: appearance.fontSize,
    volume,
    muted,
    mode: profile.playMode,
    level: performance.level,
    reducedMotion: performance.reducedMotion,
    enableSearchHistory,
    historyCount,
    isRecording,
    token: savedToken,
    cookie: '',
  }
  const draft = { ...current, ...changes }
  const dirty = (Object.keys(changes) as (keyof SettingsDraft)[]).some(key => draft[key] !== current[key])
  const update = <K extends keyof SettingsDraft>(key: K, value: SettingsDraft[K]) => {
    setChanges(previous => ({ ...previous, [key]: value }))
    setError('')
  }
  const connected = useStore(
    websocketStore,
    s => s.connectionStatus === 'connected',
  )
  const save = async () => {
    if (savingRef.current || !dirty)
      return
    savingRef.current = true
    setSaving(true)
    setError('')
    const completed: (keyof SettingsDraft)[] = []
    const failures: string[] = []
    const commit = async (keys: (keyof SettingsDraft)[], label: string, action: () => void | boolean | Promise<void | boolean>) => {
      if (!keys.some(key => draft[key] !== current[key]))
        return
      try {
        if (await action() === false)
          throw new Error('保存失败，请重试')
        completed.push(...keys)
      } catch (err) {
        failures.push(`${label}：${err instanceof Error ? err.message : '保存失败，请重试'}`)
      }
    }
    try {
      await commit(['name', 'email'], '个人资料', async () => {
        const profile = { name: draft.name.trim(), email: draft.email.trim() }
        if (connected && !await userSettingsStore.syncUserSettings(profile))
          return false
        userSettingsStore.setUserName(profile.name)
        userSettingsStore.setUserEmail(profile.email)
      })
      await commit(['theme', 'fontSize'], '外观', () => appearanceStore.update(draft.theme, draft.fontSize))
      await commit(['volume'], '音量', () => playerStore.setVolume(draft.volume))
      await commit(['muted'], '静音', () => {
        if (playerStore.state.isMuted !== draft.muted)
          playerStore.toggleMute()
      })
      await commit(['mode'], '播放模式', () => {
        if (websocketStore.state.connectionStatus !== 'connected')
          throw new Error('连接已断开，请连接房间后重试')
        return userSettingsStore.setPlayMode(draft.mode)
      })
      await commit(['level'], '性能模式', () => performanceActions[draft.level]())
      await commit(['reducedMotion'], '减少动态效果', () => {
        if (performanceStore.state.reducedMotion !== draft.reducedMotion)
          performanceStore.toggleReducedMotion()
      })
      await commit(['enableSearchHistory', 'historyCount'], '搜索记录', () => historyStore.updateSearchHistorySettings(draft.enableSearchHistory, draft.historyCount))
      await commit(['isRecording'], '播放历史', () => {
        if (historyStore.state.isRecording !== draft.isRecording)
          historyStore.togglePlayHistoryRecording()
      })
      await commit(['token', 'cookie'], '网易云 Cookie', async () => {
        if (draft.cookie.trim()) {
          if (!draft.token.trim())
            throw new Error('请输入管理 Token')
          const data = await requestCookie(draft.token, draft.cookie)
          setCookieStatus('已设置')
          if (!data.persisted)
            notificationStore.warning('Cookie 已生效，写入配置文件失败，重启后会丢失')
        }
        localStorage.setItem('alisten-cookie-admin-token', draft.token.trim())
        setSavedToken(draft.token.trim())
      })
      setChanges((previous) => {
        const pending = { ...previous }
        for (const key of completed)
          delete pending[key]
        return pending
      })
      if (failures.length)
        setError(failures.join('；'))
      else
        notificationStore.success('设置已保存')
    } finally {
      savingRef.current = false
      setSaving(false)
    }
  }
  const unlockCookieSettings = () => {
    if (cookieUnlocked)
      return
    const now = window.performance.now()
    const clicks = unlockClicks.current
    clicks.count = now - clicks.lastClickAt > 2000 ? 1 : clicks.count + 1
    clicks.lastClickAt = now
    if (clicks.count >= 7) {
      setCookieUnlocked(true)
      notificationStore.success('已开启网易云 Cookie 设置')
    }
  }
  return (
    <section className="settings-page" aria-labelledby="settings-title">
      <header className="settings-page-header">
        <h1 id="settings-title" aria-label="设置" className="text-ui-xl" tabIndex={-1} ref={heading}>
          <button type="button" className="settings-title-trigger" aria-label="设置标题" onClick={unlockCookieSettings}>设置</button>
        </h1>
      </header>
      <form
        className="settings-page-form"
        onSubmit={(event) => {
          event.preventDefault()
          void save()
        }}
      >
        <div className="settings-page-body">
          <fieldset className="settings-fields" disabled={saving}>
            <div className="settings-content">
              <section className="settings-section" aria-labelledby="settings-profile-title">
                <h3 id="settings-profile-title" className="text-ui-lg">个人资料</h3>
                <div className="settings-group">
                  <SettingRow id="settings-name" title="昵称" description="房间成员看到的名字。" field>
                    <input id="settings-name" aria-describedby="settings-name-description" value={draft.name} onChange={e => update('name', e.target.value)} placeholder="匿名用户" maxLength={40} autoComplete="nickname" />
                  </SettingRow>
                  <SettingRow id="settings-email" title="邮箱" description="用于获取 Gravatar 头像。" field>
                    <input id="settings-email" aria-describedby="settings-email-description" type="email" value={draft.email} onChange={e => update('email', e.target.value)} placeholder="用于 Gravatar 头像" autoComplete="email" />
                  </SettingRow>
                </div>
              </section>
              <section className="settings-section" aria-labelledby="settings-appearance-title">
                <h3 id="settings-appearance-title" className="text-ui-lg">外观</h3>
                <div className="settings-group">
                  <SettingRow id="settings-theme" title="主题" description="选择界面的明暗外观。">
                    <select id="settings-theme" aria-describedby="settings-theme-description" value={draft.theme} onChange={e => update('theme', e.target.value as Theme)}>
                      <option value="dark">深色</option>
                      <option value="light">浅色</option>
                      <option value="system">跟随系统</option>
                    </select>
                  </SettingRow>
                  <SettingRow id="settings-font-size" title="界面字号" description="调整文字大小，布局间距保持不变。">
                    <select id="settings-font-size" aria-describedby="settings-font-size-description" value={draft.fontSize} onChange={e => update('fontSize', Number(e.target.value))}>
                      {[12, 14, 16, 18].map(size => (
                        <option key={size} value={size}>
                          {size}
                          {' '}
                          px
                        </option>
                      ))}
                    </select>
                  </SettingRow>
                </div>
              </section>
              <section className="settings-section" aria-labelledby="settings-playback-title">
                <h3 id="settings-playback-title" className="text-ui-lg">播放</h3>
                <div className="settings-group">
                  <SettingRow id="settings-play-mode" title="播放模式" description={connected ? '更改当前房间的歌曲播放顺序。' : '加入房间后可更改播放模式。'}>
                    <select id="settings-play-mode" aria-describedby="settings-play-mode-description" value={draft.mode} disabled={!connected} onChange={e => update('mode', e.target.value === 'random' ? 'random' : 'sequential')}>
                      <option value="sequential">顺序播放</option>
                      <option value="random">随机播放</option>
                    </select>
                  </SettingRow>
                  <SettingRow id="settings-volume" title="音量" description="仅调整当前设备的播放音量。" field>
                    <div className="settings-volume-control">
                      <input id="settings-volume" aria-describedby="settings-volume-description" type="range" min="0" max="100" value={draft.volume} onChange={e => update('volume', Number(e.target.value))} />
                      <output htmlFor="settings-volume" className="text-ui-caption muted">
                        {Math.round(draft.volume)}
                        %
                      </output>
                    </div>
                  </SettingRow>
                  <SettingRow id="settings-muted" title="静音" description="暂时关闭当前设备的声音。">
                    <input id="settings-muted" aria-describedby="settings-muted-description" className="settings-switch" type="checkbox" role="switch" checked={draft.muted} onChange={e => update('muted', e.target.checked)} />
                  </SettingRow>
                </div>
              </section>
              <section className="settings-section" aria-labelledby="settings-performance-title">
                <h3 id="settings-performance-title" className="text-ui-lg">性能</h3>
                <div className="settings-group">
                  <SettingRow id="settings-performance" title="性能模式" description={performanceStore.getDescription(draft.level)}>
                    <select id="settings-performance" aria-describedby="settings-performance-description" value={draft.level} onChange={e => update('level', e.target.value as PerformanceLevel)}>
                      <option value="high">高质量</option>
                      <option value="medium">平衡</option>
                      <option value="low">省电</option>
                      <option value="off">极简</option>
                    </select>
                  </SettingRow>
                  <SettingRow id="settings-reduced-motion" title="减少动态效果" description="关闭界面动画和歌词平滑滚动。">
                    <input id="settings-reduced-motion" aria-describedby="settings-reduced-motion-description" className="settings-switch" type="checkbox" role="switch" checked={draft.reducedMotion} onChange={e => update('reducedMotion', e.target.checked)} />
                  </SettingRow>
                </div>
              </section>
              <section className="settings-section" aria-labelledby="settings-history-title">
                <h3 id="settings-history-title" className="text-ui-lg">历史记录</h3>
                <div className="settings-group">
                  <SettingRow id="settings-search-history" title="保存搜索记录" description="在此设备上保留最近搜索的内容。">
                    <input id="settings-search-history" aria-describedby="settings-search-history-description" className="settings-switch" type="checkbox" role="switch" checked={draft.enableSearchHistory} onChange={e => update('enableSearchHistory', e.target.checked)} />
                  </SettingRow>
                  <SettingRow id="settings-history-count" title="搜索记录数量" description="设置最多保留的搜索记录条数。">
                    <select id="settings-history-count" aria-describedby="settings-history-count-description" value={draft.historyCount} disabled={!draft.enableSearchHistory} onChange={e => update('historyCount', Number(e.target.value))}>
                      {[5, 10, 20, 50].map(n => (
                        <option value={n} key={n}>
                          {n}
                          {' '}
                          条
                        </option>
                      ))}
                    </select>
                  </SettingRow>
                  <SettingRow id="settings-play-history" title="记录播放历史" description="将听过的歌曲保存在此设备上。">
                    <input id="settings-play-history" aria-describedby="settings-play-history-description" className="settings-switch" type="checkbox" role="switch" checked={draft.isRecording} onChange={e => update('isRecording', e.target.checked)} />
                  </SettingRow>
                </div>
              </section>
              {cookieUnlocked && <CookieSettings token={draft.token} cookie={draft.cookie} status={cookieStatus} onTokenChange={value => update('token', value)} onCookieChange={value => update('cookie', value)} onStatusChange={setCookieStatus} />}
            </div>
          </fieldset>
        </div>
        <footer className="settings-page-footer">
          <div className="settings-save-status">
            {error
              ? <p className="error-text text-ui-caption" role="alert">{error}</p>
              : <p className="muted text-ui-caption">{dirty ? '有未保存的更改' : '所有更改已保存'}</p>}
          </div>
          <button type="submit" className="button-primary" disabled={!dirty || saving} aria-busy={saving}>{saving ? '保存中…' : '保存'}</button>
        </footer>
      </form>
    </section>
  )
}

function SettingRow({ id, title, description, field = false, children }: {
  id: string
  title: string
  description: string
  field?: boolean
  children: ReactNode
}) {
  return (
    <div className={`settings-row ${field ? 'settings-row-field' : ''}`}>
      <div className="settings-row-copy">
        <label htmlFor={id} className="settings-row-title text-ui-base">{title}</label>
        <p id={`${id}-description`} className="text-ui-caption muted">{description}</p>
      </div>
      <div className="settings-row-control">{children}</div>
    </div>
  )
}

async function requestCookie(token: string, cookie?: string): Promise<{ set?: boolean, persisted?: boolean }> {
  const url = `${getAppConfig().api.baseUrl.replace(/\/+$/, '')}/config/cookie`
  const response = await fetch(url, {
    method: cookie === undefined ? 'GET' : 'POST',
    headers: {
      'Authorization': `Bearer ${token.trim()}`,
      'Content-Type': 'application/json',
    },
    ...(cookie === undefined ? {} : { body: JSON.stringify({ cookie: cookie.trim() }) }),
  })
  if (!response.ok) {
    throw new Error(response.status === 401
      ? 'Token 错误'
      : response.status === 403 ? '服务端未配置 Token' : `请求失败 (HTTP ${response.status})`)
  }
  return response.json()
}

function CookieSettings({ token, cookie, status, onTokenChange, onCookieChange, onStatusChange }: {
  token: string
  cookie: string
  status: string
  onTokenChange: (value: string) => void
  onCookieChange: (value: string) => void
  onStatusChange: (value: string) => void
}) {
  const section = useRef<HTMLElement>(null)
  useEffect(() => {
    section.current?.scrollIntoView({ block: 'start', behavior: 'instant' })
  }, [])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const request = async () => {
    if (busy || !token.trim())
      return
    setBusy(true)
    setError('')
    try {
      const data = await requestCookie(token)
      onStatusChange(data.set ? '已设置' : '未设置')
    } catch (err) {
      setError(err instanceof Error ? err.message : '无法连接服务器')
    } finally {
      setBusy(false)
    }
  }
  return (
    <section ref={section} className="settings-section" aria-labelledby="settings-cookie-title">
      <h3 id="settings-cookie-title" className="text-ui-lg">网易云 Cookie</h3>
      <div className="settings-group">
        <SettingRow id="settings-admin-token" title="管理 Token" description="管理员验证身份后可更新服务端 Cookie。" field>
          <input
            id="settings-admin-token"
            aria-describedby="settings-admin-token-description"
            type="password"
            value={token}
            onChange={e => onTokenChange(e.target.value)}
            autoComplete="off"
            placeholder="输入管理 Token"
          />
        </SettingRow>
        <div className="settings-row">
          <div className="settings-row-copy">
            <span className="settings-row-title text-ui-base">Cookie 状态</span>
            <p className="text-ui-caption muted" role="status" aria-label="Cookie 状态">{status}</p>
          </div>
          <div className="settings-row-control">
            <button
              type="button"
              className="button-secondary"
              disabled={!token.trim() || busy}
              onClick={() => void request()}
            >
              {busy ? '处理中…' : '查询状态'}
            </button>
          </div>
        </div>
        <SettingRow id="settings-cookie" title="Cookie" description="粘贴新的网易云音乐 Cookie 并保存至服务端。" field>
          <textarea
            id="settings-cookie"
            aria-describedby="settings-cookie-description"
            value={cookie}
            onChange={e => onCookieChange(e.target.value)}
            rows={3}
            placeholder="粘贴网易云音乐 Cookie"
          />
        </SettingRow>
        {error && (
          <p className="settings-error error-text text-ui-caption" role="alert">
            {error}
          </p>
        )}

      </div>
    </section>
  )
}
