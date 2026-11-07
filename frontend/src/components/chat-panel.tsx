import type { ScrollRef, ViewRef } from 'nsh'
import { createMotion, effect, Overlay, Scroll, signal, Text, View, windowSize } from 'nsh'
import { Input } from 'nsh/base'
import { useStore } from '@/hooks/use-store'
import { chatStore } from '@/stores/chat-store'
import { userSettingsStore } from '@/stores/user-settings-store'
import { websocketStore } from '@/stores/websocket-store'
import { accent, brand, compact, font, foreground, input, inputBorder, muted, subtle, surface } from '@/theme'
import { Artwork, EmptyState, IconButton } from './ui'

export function ChatPanel(props: { onClose?: () => void }) {
  const messages = useStore(chatStore, state => state.chatMessages)
  const users = useStore(chatStore, state => state.onlineUsers)
  const userName = useStore(userSettingsStore, state => state.userName)
  const connected = useStore(websocketStore, state => state.connectionStatus === 'connected')
  const draft = signal('')
  const showUsers = signal(false)
  const memberMotion = createMotion()
  effect(() => {
    if (showUsers()) {
      void memberMotion.play({
        duration: 200,
        keyframes: [
          { opacity: 0, translateY: -8, scale: 0.96 },
          { opacity: 1, translateY: 0, scale: 1 },
        ],
      })
    }
  })
  const visibleCount = signal(80)
  const readCount = signal(messages().length)
  const atBottom = signal(true)
  let pinned = true
  let contentHeight = 0
  let list: ScrollRef | undefined
  let members: ViewRef | undefined
  const first = () => Math.max(0, messages().length - visibleCount())

  const scrollToLatest = () => {
    list?.scrollTo({ y: contentHeight, animated: false })
    pinned = true
    readCount.set(messages().length)
    atBottom.set(true)
  }
  effect(() => {
    messages()
    if (pinned)
      requestAnimationFrame(scrollToLatest)
  })
  const send = () => {
    if (connected() && websocketStore.sendChatMessage(draft()))
      draft.set('')
  }

  return (
    <View accessibilityLabel="房间聊天" style={{ flex: 1, minWidth: 0, minHeight: 0 }}>
      <View style={{ minHeight: 64, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12, paddingHorizontal: 20, paddingVertical: 14, borderBottomWidth: 1, borderBottomColor: '#333337' }}>
        <Text style={[font(4), { color: foreground, fontWeight: '600' }]}>房间聊天</Text>
        <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
          <View ref={members}>
            <View role="button" class="cursor-pointer" accessibilityLabel={`在线成员，${users().length} 人`} onPress={() => showUsers.update(value => !value)} style={{ flexDirection: 'row', alignItems: 'center', gap: 6, padding: 8, borderRadius: 8 }}>
              <Text style={[font(-2), { color: showUsers() ? brand : muted }]}>{`${users().length} 人`}</Text>
            </View>
            <If when={showUsers()}>
              <Overlay
                anchor={members}
                placement="bottom"
                align="end"
                offset={10}
                onPressOutside={() => showUsers.set(false)}
                onCancel={() => showUsers.set(false)}
                style={[{ width: Math.min(272, windowSize().width - 32), maxWidth: '100%', padding: 8, borderWidth: 1, borderColor: '#333337', borderRadius: 12, backgroundColor: '#242427', boxShadow: '0 16px 40px rgba(0,0,0,0.24)', transformOrigin: 'top right' }, memberMotion.style]}
              >
                <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: 8, paddingTop: 6, paddingBottom: 10 }}>
                  <Text style={[font(-2), { color: foreground, fontWeight: '600' }]}>在线成员</Text>
                  <Text style={[font(-4), { color: muted }]}>{`${users().length} 人`}</Text>
                </View>
                <Scroll style={{ maxHeight: 280 }} contentStyle={{ gap: 2 }}>
                  <Each of={users()} key={user => `${user.name}:${user.emailHash || user.avatar}`}>
                    {user => (
                      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 8, paddingVertical: 6, borderRadius: 8 }}>
                        <Artwork src={user().avatar} title={user().name} size={28} radius={14} />
                        <Text style={[font(-2), { color: foreground, flex: 1, numberOfLines: 1 }]}>{user().name}</Text>
                      </View>
                    )}
                  </Each>
                  <If when={!users().length}>
                    <Text style={[font(-2), { color: muted, padding: 8 }]}>连接后显示在线成员</Text>
                  </If>
                </Scroll>
              </Overlay>
            </If>
          </View>
          <If when={props.onClose}>
            <IconButton icon="X" label="关闭侧栏" onPress={props.onClose} />
          </If>
        </View>
      </View>
      <Scroll
        ref={list}
        onScroll={({ offset, contentSize, size }) => {
          contentHeight = contentSize.height
          pinned = contentSize.height - offset.y - size.height < 60
          atBottom.set(pinned)
          if (pinned)
            readCount.set(messages().length)
        }}
        style={{ flex: 1, minHeight: 0 }}
        contentStyle={{ flexGrow: 1, padding: compact() ? 12 : 20, gap: 20 }}
      >
        <If when={first() > 0}>
          <TextButtonLike label="查看更早的消息" onPress={() => visibleCount.update(value => value + 80)} />
        </If>
        <If when={!messages().length}>
          <EmptyState icon="MessageCircle" title="好音乐，一起分享">打个招呼，或分享这首歌带给你的心情。</EmptyState>
        </If>
        <Each of={messages().slice(first())} key={message => `${message.timestamp}:${message.user.name}:${message.content}`}>
          {message => (
            <View style={{ flexDirection: 'row', gap: 12, flexShrink: 0 }}>
              <Artwork src={message().user.avatar} title={message().user.name} size={32} radius={16} />
              <View style={{ flex: 1, minWidth: 0 }}>
                <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                  <Text style={[font(-4), { color: muted, minWidth: 0, flexShrink: 1, numberOfLines: 1 }]}>{message().user.name}</Text>
                  <Text style={[font(-4), { color: muted, flexShrink: 0, fontVariant: 'tabular-nums' }]}>{new Date(message().timestamp).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</Text>
                </View>
                <View style={{ alignSelf: 'flex-start', maxWidth: '100%', paddingHorizontal: 12, paddingVertical: 8, borderRadius: 8, backgroundColor: message().user.name === (userName() || '匿名用户') ? accent : surface }}>
                  <Text style={[font(), { color: foreground, lineHeight: (font().fontSize ?? 14) * 1.65 }]}>{message().content}</Text>
                </View>
              </View>
            </View>
          )}
        </Each>
      </Scroll>
      <If when={!atBottom() && messages().length > readCount()}>
        <View role="button" class="cursor-pointer" accessibilityLabel="查看新消息" onPress={scrollToLatest} style={{ alignSelf: 'center', marginBottom: 8, paddingHorizontal: 12, paddingVertical: 6, borderRadius: 8, backgroundColor: brand }}>
          <Text style={[font(-2), { color: '#122412' }]}>查看新消息</Text>
        </View>
      </If>
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, flexShrink: 0, marginHorizontal: compact() ? 12 : 20, marginBottom: compact() ? 12 : 20, padding: 6, borderWidth: 1, borderColor: inputBorder, borderRadius: 12, backgroundColor: input }}>
        <Input
          accessibilityLabel="聊天消息"
          placeholder={connected() ? '聊聊这首歌…' : '连接房间后开始聊天'}
          value={draft()}
          onChange={({ value }) => draft.set(value)}
          onSubmit={send}
          returnKeyType="send"
          submitBehavior="submit"
          maxLength={500}
          disabled={!connected()}
          style={{ flex: 1, minWidth: 0, padding: 8, color: foreground, placeholderColor: subtle, fontSize: compact() ? Math.max(16, font().fontSize || 14) : font().fontSize }}
        />
        <IconButton icon="Send" label="发送消息" color={brand} disabled={!connected() || !draft().trim()} onPress={send} />
      </View>
    </View>
  )
}

function TextButtonLike(props: { label: string, onPress: () => void }) {
  return (
    <View role="button" class="cursor-pointer" accessibilityLabel={props.label} onPress={props.onPress} style={{ alignSelf: 'center', padding: 8, borderRadius: 8 }}>
      <Text style={[font(-2), { color: brand }]}>{props.label}</Text>
    </View>
  )
}
