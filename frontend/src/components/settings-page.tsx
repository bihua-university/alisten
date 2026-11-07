import type { JSX } from 'nsh'
import type { Theme } from '@/stores/appearance-store'
import type { PerformanceLevel } from '@/stores/performance-store'
import type { PlayMode } from '@/types'
import { computed, Scroll, signal, Text, View, windowSize } from 'nsh'
import { Input, Slider, Switch } from 'nsh/base'
import { useStore } from '@/hooks/use-store'
import { appearanceStore } from '@/stores/appearance-store'
import { historyStore } from '@/stores/history-store'
import { notificationStore } from '@/stores/notification-store'
import { performanceStore } from '@/stores/performance-store'
import { playerStore } from '@/stores/player-store'
import { userSettingsStore } from '@/stores/user-settings-store'
import { websocketStore } from '@/stores/websocket-store'
import { brand, compact, content, font, palette } from '@/theme'
import { getAppConfig } from '@/utils/config'
import { applyEnvironment, environmentStorageKey } from '@/utils/environment'
import { localStorage } from '@/utils/storage'
import { SelectField } from './select-field'
import { TextButton } from './ui'

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
  const unlock = { count: 0, lastClickAt: 0 }
  const cookieUnlocked = signal(false)
  const profile = useStore(userSettingsStore, state => state)
  const performanceState = useStore(performanceStore, state => state)
  const appearance = useStore(appearanceStore, state => state)
  const volume = useStore(playerStore, state => state.volume)
  const muted = useStore(playerStore, state => state.isMuted)
  const enableSearchHistory = useStore(historyStore, state => state.enableSearchHistory)
  const historyCount = useStore(historyStore, state => state.maxSearchHistoryCount)
  const isRecording = useStore(historyStore, state => state.isRecording)
  const connected = useStore(websocketStore, state => state.connectionStatus === 'connected')
  const savedToken = signal(localStorage.getItem(environmentStorageKey('alisten-cookie-admin-token')) || '')
  const changes = signal<Partial<SettingsDraft>>({})
  const saving = signal(false)
  const error = signal('')
  const cookieStatus = signal('未查询')
  const contentWidth = signal(windowSize().width)
  const current = (): SettingsDraft => ({
    name: profile().userName,
    email: profile().userEmail,
    theme: appearance().theme,
    fontSize: appearance().fontSize,
    volume: volume(),
    muted: muted(),
    mode: profile().playMode,
    level: performanceState().level,
    reducedMotion: performanceState().reducedMotion,
    enableSearchHistory: enableSearchHistory(),
    historyCount: historyCount(),
    isRecording: isRecording(),
    token: savedToken(),
    cookie: '',
  })
  const draft = () => ({ ...current(), ...changes() })
  const dirty = computed(() => (Object.keys(changes()) as (keyof SettingsDraft)[]).some(key => draft()[key] !== current()[key]))
  const update = <K extends keyof SettingsDraft>(key: K, value: SettingsDraft[K]) => {
    changes.update(previous => ({ ...previous, [key]: value }))
    error.set('')
  }

  const save = async () => {
    if (saving() || !dirty())
      return
    saving.set(true)
    error.set('')
    const before = current()
    const values = draft()
    const completed: (keyof SettingsDraft)[] = []
    const failures: string[] = []
    const commit = async (keys: (keyof SettingsDraft)[], label: string, action: () => void | boolean | Promise<void | boolean>) => {
      if (!keys.some(key => values[key] !== before[key]))
        return
      try {
        if (await action() === false)
          throw new Error('保存失败，请重试')
        completed.push(...keys)
      } catch (reason) {
        failures.push(`${label}：${reason instanceof Error ? reason.message : '保存失败，请重试'}`)
      }
    }
    try {
      await commit(['name', 'email'], '个人资料', async () => {
        const next = { name: values.name.trim(), email: values.email.trim() }
        if (connected() && !await userSettingsStore.syncUserSettings(next))
          return false
        userSettingsStore.setUserName(next.name)
        userSettingsStore.setUserEmail(next.email)
      })
      await commit(['theme', 'fontSize'], '外观', () => appearanceStore.update(values.theme, values.fontSize))
      await commit(['volume'], '音量', () => playerStore.setVolume(values.volume))
      await commit(['muted'], '静音', () => {
        if (playerStore.state.isMuted !== values.muted)
          playerStore.toggleMute()
      })
      await commit(['mode'], '播放模式', () => {
        if (websocketStore.state.connectionStatus !== 'connected')
          throw new Error('连接已断开，请连接房间后重试')
        return userSettingsStore.setPlayMode(values.mode)
      })
      await commit(['level'], '性能模式', () => performanceActions[values.level]())
      await commit(['reducedMotion'], '减少动态效果', () => {
        if (performanceStore.state.reducedMotion !== values.reducedMotion)
          performanceStore.toggleReducedMotion()
      })
      await commit(['enableSearchHistory', 'historyCount'], '搜索记录', () => historyStore.updateSearchHistorySettings(values.enableSearchHistory, values.historyCount))
      await commit(['isRecording'], '播放历史', () => {
        if (historyStore.state.isRecording !== values.isRecording)
          historyStore.togglePlayHistoryRecording()
      })
      await commit(['token', 'cookie'], '网易云 Cookie', async () => {
        if (values.cookie.trim()) {
          if (!values.token.trim())
            throw new Error('请输入管理 Token')
          const data = await requestCookie(values.token, values.cookie)
          cookieStatus.set('已设置')
          if (!data.persisted)
            notificationStore.warning('Cookie 已生效，写入配置文件失败，重启后会丢失')
        }
        localStorage.setItem(environmentStorageKey('alisten-cookie-admin-token'), values.token.trim())
        savedToken.set(values.token.trim())
      })
      changes.update((previous) => {
        const pending = { ...previous }
        for (const key of completed)
          delete pending[key]
        return pending
      })
      if (failures.length)
        error.set(failures.join('；'))
      else
        notificationStore.success('设置已保存')
    } finally {
      saving.set(false)
    }
  }

  const unlockCookieSettings = () => {
    if (cookieUnlocked())
      return
    const now = performance.now()
    unlock.count = now - unlock.lastClickAt > 2000 ? 1 : unlock.count + 1
    unlock.lastClickAt = now
    if (unlock.count >= 7) {
      cookieUnlocked.set(true)
      notificationStore.success('已开启网易云 Cookie 设置')
    }
  }
  const narrow = () => contentWidth() <= 640
  const colors = palette('dark')

  return (
    <View accessibilityLabel="设置" style={{ flex: 1, minHeight: 0, backgroundColor: content }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: narrow() ? 20 : 40, paddingTop: narrow() ? 20 : 28, paddingBottom: narrow() ? 16 : 20 }}>
        <View role="button" class="cursor-pointer" accessibilityLabel="设置标题" onPress={unlockCookieSettings}>
          <Text style={[font(14), { color: colors.foreground, fontWeight: '600' }]}>设置</Text>
        </View>
      </View>
      <Scroll
        keyboardShouldPersistTaps="handled"
        onLayout={({ bounds }) => contentWidth.set(bounds.size.width)}
        style={{ flex: 1, minHeight: 0 }}
        contentStyle={{ paddingHorizontal: narrow() ? 16 : 40, paddingTop: 16, paddingBottom: 48, gap: narrow() ? 28 : 40 }}
      >
        <SettingsSection title="个人资料">
          <SettingRow title="昵称" description="房间成员看到的名字。" field narrow={narrow()}>
            <TextInput value={draft().name} onChange={value => update('name', value)} placeholder="匿名用户" maxLength={40} autoComplete="nickname" />
          </SettingRow>
          <SettingRow title="邮箱" description="用于获取 Gravatar 头像。" field narrow={narrow()}>
            <TextInput value={draft().email} onChange={value => update('email', value)} placeholder="用于 Gravatar 头像" keyboardType="email-address" autoComplete="email" />
          </SettingRow>
        </SettingsSection>
        <SettingsSection title="外观">
          <SettingRow title="主题" description="选择界面的明暗外观。" narrow={narrow()}>
            <SelectField label="主题" fullWidth value={draft().theme} items={[{ value: 'dark', title: '深色' }, { value: 'light', title: '浅色' }, { value: 'system', title: '跟随系统' }]} onChange={value => update('theme', value as Theme)} />
          </SettingRow>
          <SettingRow title="界面字号" description="调整文字大小，布局间距保持不变。" narrow={narrow()}>
            <SelectField label="界面字号" fullWidth value={String(draft().fontSize)} items={[12, 14, 16, 18].map(size => ({ value: String(size), title: `${size} px` }))} onChange={value => update('fontSize', Number(value))} />
          </SettingRow>
        </SettingsSection>
        <SettingsSection title="播放">
          <SettingRow title="播放模式" description={connected() ? '更改当前房间的歌曲播放顺序。' : '加入房间后可更改播放模式。'} narrow={narrow()}>
            <SelectField label="播放模式" fullWidth value={draft().mode} disabled={!connected()} items={[{ value: 'sequential', title: '顺序播放' }, { value: 'random', title: '随机播放' }]} onChange={value => update('mode', value as PlayMode)} />
          </SettingRow>
          <SettingRow title="音量" description="仅调整当前设备的播放音量。" field narrow={narrow()}>
            <View style={{ flex: 1, minWidth: 0, flexDirection: 'row', alignItems: 'center', gap: 16 }}>
              <Slider
                value={draft().volume}
                min={0}
                max={100}
                onChange={({ value }) => update('volume', value as number)}
                accessibilityLabel="音量"
                style={{ flex: 1, minWidth: 0, flexDirection: 'row', alignItems: 'center' }}
                trackStyle={{ height: 44, flex: 1, justifyContent: 'center' }}
                indicatorStyle={{ height: 3, borderRadius: 4, backgroundColor: colors.muted }}
                rangeStyle={{ backgroundColor: brand }}
                thumbStyle={{ width: 12, height: 12, borderRadius: 999, backgroundColor: brand }}
              />
              <Text style={[font(-1), { color: colors.muted, minWidth: 42, textAlign: 'right' }]}>{`${Math.round(draft().volume)}%`}</Text>
            </View>
          </SettingRow>
          <SettingRow title="静音" description="暂时关闭当前设备的声音。" inline narrow={narrow()}>
            <SettingsSwitch checked={draft().muted} onChange={checked => update('muted', checked)} label="静音" />
          </SettingRow>
        </SettingsSection>
        <SettingsSection title="性能">
          <SettingRow title="性能模式" description={performanceStore.getDescription(draft().level)} narrow={narrow()}>
            <SelectField label="性能模式" fullWidth value={draft().level} items={[{ value: 'high', title: '高质量' }, { value: 'medium', title: '平衡' }, { value: 'low', title: '省电' }, { value: 'off', title: '极简' }]} onChange={value => update('level', value as PerformanceLevel)} />
          </SettingRow>
          <SettingRow title="减少动态效果" description="关闭界面动画和歌词平滑滚动。" inline narrow={narrow()}>
            <SettingsSwitch checked={draft().reducedMotion} onChange={checked => update('reducedMotion', checked)} label="减少动态效果" />
          </SettingRow>
        </SettingsSection>
        <SettingsSection title="历史记录">
          <SettingRow title="保存搜索记录" description="在此设备上保留最近搜索的内容。" inline narrow={narrow()}>
            <SettingsSwitch checked={draft().enableSearchHistory} onChange={checked => update('enableSearchHistory', checked)} label="保存搜索记录" />
          </SettingRow>
          <SettingRow title="搜索记录数量" description="设置最多保留的搜索记录条数。" narrow={narrow()}>
            <SelectField label="搜索记录数量" fullWidth value={String(draft().historyCount)} disabled={!draft().enableSearchHistory} items={[5, 10, 20, 50].map(count => ({ value: String(count), title: `${count} 条` }))} onChange={value => update('historyCount', Number(value))} />
          </SettingRow>
          <SettingRow title="记录播放历史" description="将听过的歌曲保存在此设备上。" inline narrow={narrow()}>
            <SettingsSwitch checked={draft().isRecording} onChange={checked => update('isRecording', checked)} label="记录播放历史" />
          </SettingRow>
        </SettingsSection>
        <If when={cookieUnlocked()}>
          <CookieSettings token={draft().token} cookie={draft().cookie} status={cookieStatus()} narrow={narrow()} onTokenChange={value => update('token', value)} onCookieChange={value => update('cookie', value)} onStatusChange={value => cookieStatus.set(value)} />
        </If>
      </Scroll>
      <View style={{ flexShrink: 0, flexDirection: narrow() ? 'column' : 'row', alignItems: narrow() ? 'stretch' : 'center', justifyContent: 'flex-end', gap: narrow() ? 10 : 20, paddingHorizontal: narrow() ? 16 : 40, paddingTop: 12, paddingBottom: narrow() ? 12 : 24 }}>
        <Text style={[font(-1), { color: error() ? colors.destructive : colors.muted, flex: narrow() ? undefined : 1, minWidth: 0 }]}>{error() || (dirty() ? '有未保存的更改' : '所有更改已保存')}</Text>
        <TextButton title={saving() ? '保存中…' : '保存'} disabled={!dirty() || saving()} onPress={() => void save()} style={{ minWidth: 96, width: narrow() ? '100%' : undefined }} />
      </View>
    </View>
  )
}

function SettingsSection(props: { title: string, children: JSX.Element }) {
  const colors = palette('dark')
  return (
    <View style={{ maxWidth: 1040, width: '100%', marginHorizontal: 'auto' }}>
      <Text style={[font(4), { color: colors.foreground, fontWeight: '600', marginBottom: 16 }]}>{props.title}</Text>
      <View style={{ paddingHorizontal: compact() ? 16 : 20, borderWidth: 1, borderColor: colors.border, borderRadius: 12, backgroundColor: '#1F1F1F' }}>
        {props.children}
      </View>
    </View>
  )
}

function SettingRow(props: { title: string, description: string, children: JSX.Element, field?: boolean, narrow?: boolean, inline?: boolean }) {
  const colors = palette('dark')
  const stacked = () => props.narrow && !props.inline
  return (
    <View style={{ flexDirection: stacked() ? 'column' : 'row', alignItems: stacked() ? 'stretch' : 'center', gap: stacked() ? 12 : props.narrow ? 16 : 24, minHeight: 86, paddingVertical: 18, borderBottomWidth: 1, borderBottomColor: colors.border }}>
      <View style={{ flex: stacked() ? undefined : 1, minWidth: 0, gap: 4 }}>
        <Text style={[font(), { color: colors.foreground, fontWeight: '600' }]}>{props.title}</Text>
        <Text style={[font(-1), { color: colors.muted }]}>{props.description}</Text>
      </View>
      <View style={{ width: stacked() ? '100%' : props.inline ? 52 : props.field ? 280 : 176, maxWidth: '100%', minWidth: 0, flexShrink: 0, flexDirection: 'row', justifyContent: 'flex-end', alignItems: 'center' }}>
        {props.children}
      </View>
    </View>
  )
}

function TextInput(props: {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  maxLength?: number
  keyboardType?: 'email-address'
  autoComplete?: string
  masked?: boolean
  multiline?: boolean
}) {
  const colors = palette('dark')
  return (
    <Input
      value={props.value}
      onChange={({ value }) => props.onChange(value)}
      placeholder={props.placeholder}
      maxLength={props.maxLength}
      keyboardType={props.keyboardType}
      autoComplete={props.autoComplete as any}
      masked={props.masked}
      multiline={props.multiline}
      style={{ flex: 1, minWidth: 0, minHeight: props.multiline ? 104 : 48, paddingHorizontal: 12, paddingVertical: 12, borderWidth: 1, borderColor: colors.inputBorder, borderRadius: 8, backgroundColor: colors.input, color: colors.foreground, placeholderColor: colors.subtle, fontSize: compact() ? Math.max(16, font().fontSize || 14) : font().fontSize }}
    />
  )
}

function SettingsSwitch(props: { checked: boolean, onChange: (checked: boolean) => void, label: string }) {
  const colors = palette('dark')
  return (
    <Switch
      checked={props.checked}
      onChange={({ checked }) => props.onChange(checked)}
      accessibilityLabel={props.label}
      style={{ width: 52, minHeight: 44, alignItems: 'flex-end', justifyContent: 'center' }}
      trackStyle={{ width: 40, height: 24, padding: 2, borderRadius: 999, backgroundColor: props.checked ? brand : colors.selected }}
      thumbStyle={{ width: 20, height: 20, borderRadius: 999, backgroundColor: props.checked ? '#122412' : colors.foreground, translate: props.checked ? [16, 0] : [0, 0] }}
    />
  )
}

async function requestCookie(token: string, cookie?: string): Promise<{ set?: boolean, persisted?: boolean }> {
  const url = applyEnvironment(new URL('/config/cookie', getAppConfig().api.baseUrl))
  const response = await fetch(url, {
    method: cookie === undefined ? 'GET' : 'POST',
    headers: { 'Authorization': `Bearer ${token.trim()}`, 'Content-Type': 'application/json' },
    ...(cookie === undefined ? {} : { body: JSON.stringify({ cookie: cookie.trim() }) }),
  })
  if (!response.ok)
    throw new Error(response.status === 401 ? 'Token 错误' : response.status === 403 ? '服务端未配置 Token' : `请求失败 (HTTP ${response.status})`)
  return response.json()
}

function CookieSettings(props: {
  token: string
  cookie: string
  status: string
  narrow?: boolean
  onTokenChange: (value: string) => void
  onCookieChange: (value: string) => void
  onStatusChange: (value: string) => void
}) {
  const busy = signal(false)
  const error = signal('')
  const request = async () => {
    if (busy() || !props.token.trim())
      return
    busy.set(true)
    error.set('')
    try {
      const data = await requestCookie(props.token)
      props.onStatusChange(data.set ? '已设置' : '未设置')
    } catch (reason) {
      error.set(reason instanceof Error ? reason.message : '无法连接服务器')
    } finally {
      busy.set(false)
    }
  }
  return (
    <SettingsSection title="网易云 Cookie">
      <SettingRow title="管理 Token" description="管理员验证身份后可更新服务端 Cookie。" field narrow={props.narrow}>
        <TextInput value={props.token} onChange={props.onTokenChange} autoComplete="off" placeholder="输入管理 Token" masked />
      </SettingRow>
      <SettingRow title="Cookie 状态" description={props.status} narrow={props.narrow}>
        <TextButton title={busy() ? '处理中…' : '查询状态'} variant="secondary" disabled={!props.token.trim() || busy()} onPress={() => void request()} />
      </SettingRow>
      <SettingRow title="Cookie" description="粘贴新的网易云音乐 Cookie 并保存至服务端。" field narrow={props.narrow}>
        <TextInput value={props.cookie} onChange={props.onCookieChange} placeholder="粘贴网易云音乐 Cookie" multiline />
      </SettingRow>
      <If when={error()}>
        <Text style={[font(-1), { color: '#ff929a', paddingBottom: 16 }]}>{error()}</Text>
      </If>
    </SettingsSection>
  )
}
