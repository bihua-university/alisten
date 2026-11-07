import type { InputRef, Signal } from 'nsh'
import type { RoomInfo } from '@/types'
import { cleanup, effect, platform, signal, Text, View } from 'nsh'
import { Input, Switch } from 'nsh/base'
import { notificationStore } from '@/stores/notification-store'
import { websocketStore } from '@/stores/websocket-store'
import { brand, card, compact, destructive, font, foreground, input, inputBorder, muted, subtle, surface, surfaceHover } from '@/theme'
import { createRoom, searchRooms } from '@/utils/api'
import { environmentStorageKey } from '@/utils/environment'
import { getLocation } from '@/utils/runtime'
import { hydrateStorageKey } from '@/utils/storage'
import { getLastJoinedRoom, getSavedRoomPassword } from '@/utils/user'
import { Icon } from './icon'
import { EmptyState, Modal, TextButton } from './ui'

export default function RoomBrowser(props: {
  onJoin: (room: RoomInfo, password?: string) => void | Promise<void>
  joined: boolean
  onJoined: () => void
}) {
  const query = signal('')
  const rooms = signal<RoomInfo[]>([])
  const loading = signal(true)
  const error = signal('')
  const selected = signal<RoomInfo | null>(null)
  const joinOpen = signal(false)
  const password = signal('')
  const creating = signal(false)
  let autoSelected = props.joined

  const select = (room: RoomInfo) => {
    selected.set(room)
    joinOpen.set(true)
    password.set(room.needPwd ? getSavedRoomPassword(room.id) || '' : '')
    if (room.needPwd) {
      void hydrateStorageKey(environmentStorageKey(`alisten_room_password_${room.id}`)).then(() => {
        password.set(getSavedRoomPassword(room.id) || '')
      })
    }
  }

  effect(() => {
    const keyword = query()
    const controller = new AbortController()
    const timer = setTimeout(async () => {
      loading.set(true)
      error.set('')
      try {
        const data = await searchRooms(keyword.trim() || undefined, controller.signal)
        if (controller.signal.aborted)
          return
        rooms.set(data)
        if (!autoSelected) {
          autoSelected = true
          const params = getLocation().searchParams
          const invitationId = params.get('houseId') || params.get('houseid') || params.get('HOUSEID')
          const id = invitationId || getLastJoinedRoom()
          const room = data.find(item => item.id === id)
          if (room) {
            const invitationPassword = params.get('housePwd') || params.get('housepwd') || params.get('HOUSEPWD')
            select(room)
            if (invitationPassword)
              password.set(invitationPassword)
          }
        }
      } catch (reason) {
        if (!controller.signal.aborted)
          error.set(reason instanceof Error ? reason.message : '无法加载房间')
      } finally {
        if (!controller.signal.aborted)
          loading.set(false)
      }
    }, keyword ? 300 : 0)
    return () => {
      clearTimeout(timer)
      controller.abort()
    }
  })

  const mobile = compact
  return (
    <View style={{ width: '100%', maxWidth: 1400, marginHorizontal: 'auto', paddingHorizontal: mobile() ? 16 : 24, paddingTop: mobile() ? 24 : 32, paddingBottom: mobile() ? 32 : 48 }}>
      <View style={{ marginBottom: 28, gap: 8 }}>
        <Text style={[font(-2), { color: brand }]}>同频</Text>
        <Text style={[font(14), { color: foreground, fontWeight: '600' }]}>好音乐，值得一起听。</Text>
        <Text style={[font(), { color: muted }]}>找到你的听歌房，与同频的人分享此刻。</Text>
      </View>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: mobile() ? 8 : 12, marginBottom: 28 }}>
        <View style={{ flex: 1, minWidth: 0, height: mobile() ? 44 : 48, flexDirection: 'row', alignItems: 'center', gap: 12, paddingHorizontal: mobile() ? 12 : 20, borderWidth: 1, borderColor: inputBorder, borderRadius: 999, backgroundColor: input }}>
          <Icon name="Search" size={18} />
          <Input
            accessibilityLabel="搜索房间"
            value={query()}
            onChange={({ value }) => query.set(value)}
            placeholder="搜索房间名称或介绍"
            returnKeyType="search"
            style={{ flex: 1, minWidth: 0, color: foreground, placeholderColor: subtle, fontSize: mobile() ? 16 : font().fontSize }}
          />
        </View>
        <TextButton
          title="创建房间"
          onPress={() => creating.set(true)}
          style={{ borderRadius: 999, paddingHorizontal: mobile() ? 16 : 20, minHeight: mobile() ? 44 : 48 }}
        />
      </View>
      <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <Text style={[font(4), { color: foreground, fontWeight: '600' }]}>正在发生的音乐</Text>
        <Text style={[font(-2), { color: muted }]}>{loading() ? '正在寻找房间' : `${rooms().length} 个房间`}</Text>
      </View>
      <If
        when={!loading() && !error() && rooms().length > 0}
        else={loading()
          ? <RoomSkeleton />
          : error()
            ? <EmptyState title="暂时无法获取房间" action={<TextButton title="重新加载" variant="secondary" onPress={() => query.update(value => `${value} `)} />}>请检查网络连接后重试。</EmptyState>
            : <EmptyState icon="Headphones" title={query() ? '没有找到这个房间' : '成为第一个开房的人'} action={<TextButton title="创建听歌房" variant="secondary" onPress={() => creating.set(true)} />}>{query() ? '换个关键词，再找找看。' : '创建一个房间，邀请朋友一起听歌。'}</EmptyState>}
      >
        <View style={{ display: 'grid', gridTemplateColumns: mobile() ? 'repeat(2, minmax(0, 1fr))' : 'repeat(auto-fill, minmax(235px, 1fr))', gap: mobile() ? 12 : 20, rowGap: mobile() ? 12 : 24 }}>
          <Each of={rooms()} key={room => room.id}>
            {room => (
              <ButtonLikeCard room={room()} mobile={mobile()} onPress={() => select(room())} />
            )}
          </Each>
        </View>
      </If>
      <If when={selected()}>
        <JoinRoom
          open={joinOpen()}
          room={selected()!}
          password={password}
          onClose={() => joinOpen.set(false)}
          onJoin={props.onJoin}
          onJoined={props.onJoined}
        />
      </If>
      <CreateRoom open={creating()} onClose={() => creating.set(false)} onJoin={props.onJoin} onJoined={props.onJoined} />
    </View>
  )
}

function ButtonLikeCard(props: { room: RoomInfo, mobile: boolean, onPress: () => void }) {
  return (
    <View
      role="button"
      accessibilityLabel={`${props.room.needPwd ? '密码 ' : ''}${props.room.name} ${props.room.description || ''} ${props.room.population} 人正在听`}
      onPress={props.onPress}
      class="group cursor-pointer transition-colors duration-150"
      focusStyle={{ outlineWidth: 2, outlineColor: brand, outlineOffset: 3 }}
      hoverStyle={{ backgroundColor: surface }}
      style={{ minWidth: 0, gap: props.mobile ? 10 : 12, padding: props.mobile ? 10 : 12, borderRadius: 12, backgroundColor: card }}
    >
      <View style={{ width: '100%', aspectRatio: props.mobile ? 1 : 1.6, alignItems: 'center', justifyContent: 'center', borderRadius: 8, backgroundColor: surface }}>
        <Icon name="Headphones" size={props.mobile ? 36 : 44} color={brand} />
        <If when={props.room.needPwd}>
          <View style={{ position: 'absolute', top: 8, left: 8, flexDirection: 'row', alignItems: 'center', gap: 4, paddingHorizontal: 10, paddingVertical: 4, borderRadius: 999, backgroundColor: '#121212' }}>
            <Icon name="LockKeyhole" size={12} />
            <Text style={[font(-4), { color: muted }]}>密码</Text>
          </View>
        </If>
        <View class={props.mobile ? '' : 'translate-y-2 opacity-0 transition-all duration-150 group-hover:translate-y-0 group-hover:opacity-100 group-focus:opacity-100'} style={{ position: 'absolute', right: props.mobile ? 6 : 8, bottom: props.mobile ? 6 : 8, width: props.mobile ? 34 : 40, height: props.mobile ? 34 : 40, borderRadius: 999, alignItems: 'center', justifyContent: 'center', backgroundColor: brand }}>
          <Icon name="ArrowRight" size={20} color="#122412" />
        </View>
      </View>
      <View style={{ gap: 4, minWidth: 0 }}>
        <Text style={[font(), { color: foreground, fontWeight: '600', numberOfLines: 1 }]}>{props.room.name}</Text>
        <Text style={[font(-1), { color: muted, minHeight: (font(-1).fontSize ?? 13) * 3, numberOfLines: 2 }]}>{props.room.description || '来这里，听见好音乐。'}</Text>
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
          <Icon name="Users" size={14} />
          <Text style={[font(-2), { color: muted }]}>{`${props.room.population} 人正在听`}</Text>
        </View>
      </View>
    </View>
  )
}

function RoomSkeleton() {
  return (
    <View style={{ display: 'grid', gridTemplateColumns: compact() ? 'repeat(2, minmax(0, 1fr))' : 'repeat(auto-fill, minmax(235px, 1fr))', gap: 20 }}>
      <Each of={Array.from({ length: 8 }, (_, index) => index)}>
        {() => (
          <View style={{ gap: 12, padding: 12, borderRadius: 12, backgroundColor: card }}>
            <View style={{ width: '100%', aspectRatio: compact() ? 1 : 1.6, borderRadius: 8, backgroundColor: surfaceHover }} />
            <View style={{ height: 14, width: '70%', borderRadius: 4, backgroundColor: surfaceHover }} />
            <View style={{ height: 12, width: '45%', borderRadius: 4, backgroundColor: surface }} />
          </View>
        )}
      </Each>
    </View>
  )
}

function JoinRoom(props: {
  room: RoomInfo
  password: Signal<string>
  open: boolean
  onClose: () => void
  onJoin: (room: RoomInfo, password?: string) => void | Promise<void>
  onJoined: () => void
}) {
  const busy = signal(false)
  const error = signal('')
  let passwordInput: InputRef | undefined
  let enterAfterClose = false
  let attempt = 0
  let disposed = false
  const finishJoin = () => {
    if (enterAfterClose && !disposed) {
      enterAfterClose = false
      props.onJoined()
    }
  }
  cleanup(() => {
    disposed = true
    attempt++
  })
  const close = () => {
    const cancelConnection = busy() || enterAfterClose
    attempt++
    enterAfterClose = false
    if (cancelConnection)
      websocketStore.disconnect()
    busy.set(false)
    props.onClose()
  }
  const join = async () => {
    if (busy())
      return
    const room = props.room
    const password = room.needPwd ? props.password().trim() : undefined
    const onJoin = props.onJoin
    const onClose = props.onClose
    if (room.needPwd && !password)
      return
    busy.set(true)
    const revision = ++attempt
    error.set('')
    try {
      await onJoin(room, password)
      if (disposed || revision !== attempt)
        return
      enterAfterClose = true
      onClose()
      // Web can dispose Dialog's onDismiss listener as open becomes false.
      // Let its controlled close render before replacing the room browser.
      if (platform.os === 'web')
        queueMicrotask(finishJoin)
    } catch (reason) {
      if (!disposed && revision === attempt)
        error.set(reason instanceof Error ? reason.message : '加入失败，请重试。')
    } finally {
      if (!disposed && revision === attempt)
        busy.set(false)
    }
  }
  return (
    <Modal
      open={props.open}
      title="加入听歌房"
      onClose={close}
      onClosed={finishJoin}
      onOpen={() => {
        // Focus only once the sheet is in its final position. Keyboard
        // avoidance must not measure the sheet mid-way through its entrance.
        if (props.room.needPwd)
          passwordInput?.focus()
      }}
      footer={(
        <View style={{ gap: 16 }}>
          <If when={props.room.needPwd}>
            <View style={{ gap: 6 }}>
              <Text style={[font(), { color: foreground }]}>房间密码</Text>
              <Input
                ref={passwordInput}
                masked
                autoComplete="current-password"
                disabled={busy()}
                returnKeyType="go"
                value={props.password()}
                onChange={({ value }) => props.password.set(value)}
                onSubmit={join}
                placeholder="输入房间密码"
                style={{ padding: 12, borderWidth: 1, borderColor: inputBorder, borderRadius: 8, backgroundColor: input, color: foreground, fontSize: compact() ? 16 : font().fontSize }}
              />
            </View>
          </If>
          <If when={error()}>
            <Text style={[font(-1), { color: destructive }]}>{error()}</Text>
          </If>
          <TextButton title={busy() ? '正在加入…' : '加入房间'} disabled={busy() || (props.room.needPwd && !props.password().trim())} onPress={join} style={{ borderRadius: 999 }} />
        </View>
      )}
    >
      <View style={{ gap: 16 }}>
        <View style={{ flexDirection: 'row', gap: 16, alignItems: 'center' }}>
          <View style={{ width: compact() ? 72 : 88, height: compact() ? 72 : 88, borderRadius: 12, backgroundColor: surface, alignItems: 'center', justifyContent: 'center' }}>
            <Icon name="Headphones" size={36} color={brand} />
          </View>
          <View style={{ flex: 1, gap: 4 }}>
            <Text style={[font(4), { color: foreground, fontWeight: '600' }]}>{props.room.name}</Text>
            <Text style={[font(-1), { color: muted }]}>{props.room.description || '来这里，听见好音乐。'}</Text>
            <Text style={[font(-2), { color: muted }]}>{`${props.room.population} 人正在听${props.room.needPwd ? ' · 需要密码' : ''}`}</Text>
          </View>
        </View>
        <Text style={[font(-1), { color: muted }]}>加入后，与房间内的所有人同步播放、点歌和聊天。</Text>
      </View>
    </Modal>
  )
}

function CreateRoom(props: {
  open: boolean
  onClose: () => void
  onJoin: (room: RoomInfo, password?: string) => void | Promise<void>
  onJoined: () => void
}) {
  const name = signal('')
  const desc = signal('')
  const locked = signal(false)
  const password = signal('')
  const busy = signal(false)
  const error = signal('')
  let enterAfterClose = false
  let disposed = false
  const finishJoin = () => {
    if (enterAfterClose && !disposed) {
      enterAfterClose = false
      props.onJoined()
    }
  }
  cleanup(() => {
    disposed = true
  })
  const submit = async () => {
    if (busy() || !name().trim() || (locked() && !password().trim()))
      return
    busy.set(true)
    error.set('')
    try {
      const response = await createRoom({ name: name().trim(), desc: desc().trim(), needPwd: locked(), password: locked() ? password().trim() : '' })
      if (disposed)
        return
      if (!response.houseId)
        throw new Error('创建失败')
      await props.onJoin({ id: response.houseId, name: name().trim(), description: desc().trim(), needPwd: locked(), population: 1, ultimate: false }, locked() ? password().trim() : undefined)
      if (disposed)
        return
      notificationStore.success('房间已创建')
      enterAfterClose = true
      props.onClose()
      if (platform.os === 'web')
        queueMicrotask(finishJoin)
    } catch (reason) {
      if (!disposed)
        error.set(reason instanceof Error ? reason.message : '创建失败，请重试')
    } finally {
      if (!disposed)
        busy.set(false)
    }
  }
  return (
    <Modal
      open={props.open}
      title="创建新房间"
      onClose={props.onClose}
      onClosed={finishJoin}
    >
      <View style={{ gap: 16 }}>
        <Field label="房间名称" value={name} placeholder="给这个房间起个名字" maxLength={50} autoFocus onSubmit={submit} />
        <Field label="房间描述" value={desc} placeholder="你想和大家听什么？" maxLength={200} multiline />
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 12 }}>
          <Switch
            checked={locked()}
            onChange={({ checked }) => locked.set(checked)}
            accessibilityLabel="设置房间密码"
            trackStyle={{ width: 40, height: 24, padding: 2, borderRadius: 999, backgroundColor: locked() ? brand : surfaceHover }}
            thumbStyle={{ width: 20, height: 20, borderRadius: 999, backgroundColor: foreground, translate: locked() ? [16, 0] : [0, 0] }}
          />
          <Text style={[font(), { color: foreground }]}>设置房间密码</Text>
        </View>
        <If when={locked()}>
          <Field label="房间密码" value={password} masked autoComplete="new-password" onSubmit={submit} />
        </If>
        <If when={error()}>
          <Text style={[font(-1), { color: '#ff929a' }]}>{error()}</Text>
        </If>
        <TextButton title={busy() ? '正在创建…' : '创建并加入'} disabled={busy() || !name().trim() || (locked() && !password().trim())} onPress={submit} style={{ borderRadius: 999 }} />
      </View>
    </Modal>
  )
}

function Field(props: {
  label: string
  value: Signal<string>
  placeholder?: string
  maxLength?: number
  masked?: boolean
  multiline?: boolean
  autoFocus?: boolean
  autoComplete?: string
  onSubmit?: () => void
}) {
  return (
    <View style={{ gap: 6 }}>
      <Text style={[font(), { color: foreground }]}>{props.label}</Text>
      <Input
        value={props.value()}
        onChange={({ value }) => props.value.set(value)}
        placeholder={props.placeholder}
        maxLength={props.maxLength}
        masked={props.masked}
        multiline={props.multiline}
        autoFocus={props.autoFocus}
        autoComplete={props.autoComplete as any}
        onSubmit={props.onSubmit}
        style={{ padding: 12, minHeight: props.multiline ? 88 : undefined, borderWidth: 1, borderColor: inputBorder, borderRadius: 8, backgroundColor: input, color: foreground, placeholderColor: subtle, fontSize: compact() ? 16 : font().fontSize }}
      />
    </View>
  )
}
