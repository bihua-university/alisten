import type { IconName } from './icon'
import { effect, Overlay, safeAreaInsets, signal, Text, View, windowSize } from 'nsh'
import { useStore } from '@/hooks/use-store'
import { notificationStore } from '@/stores/notification-store'
import { playerStore } from '@/stores/player-store'
import { brand, compact, font, foreground, muted, success, warning } from '@/theme'
import { Icon } from './icon'
import { IconButton, Modal, TextButton } from './ui'

const notificationIcons: Record<string, IconName> = {
  success: 'CheckCircle2',
  error: 'AlertCircle',
  warning: 'AlertTriangle',
  info: 'Info',
}

export function Notifications() {
  const notifications = useStore(notificationStore, state => state.notifications)
  const mobile = compact
  const insets = safeAreaInsets()
  return (
    <Overlay style={{ position: 'fixed', top: insets().top + (mobile() ? 76 : 88), left: '50%', width: Math.min(420, Math.max(0, windowSize().width - 32)), maxHeight: Math.max(0, windowSize().height - 160), translate: ['-50%', 0], gap: 8, pointerEvents: 'none', zIndex: 100 }}>
      <Each of={notifications().filter(item => item.show)} key={item => item.id}>
        {item => (
          <View
            accessibilityLabel={item().message}
            style={{ flexDirection: 'row', alignItems: 'center', gap: 12, paddingHorizontal: 16, paddingVertical: 12, borderWidth: 1, borderColor: '#333337', borderRadius: 16, backgroundColor: '#303034', boxShadow: '0 8px 24px rgba(0,0,0,0.24)', pointerEvents: 'auto' }}
          >
            <Icon name={notificationIcons[item().type || 'info']} color={item().type === 'error' ? '#ff929a' : item().type === 'success' ? success : item().type === 'warning' ? warning : brand} />
            <Text style={[font(), { color: foreground, flex: 1, minWidth: 0 }]}>{item().message}</Text>
            <IconButton icon="X" label="关闭通知" onPress={() => notificationStore.hide(item().id)} />
          </View>
        )}
      </Each>
    </Overlay>
  )
}

export function SystemPrompts(props: { enabled: boolean }) {
  const manual = useStore(playerStore, state => state.needManualStartPlay)
  const dismissed = signal(false)
  const close = () => dismissed.set(true)
  effect(() => {
    if (!manual())
      dismissed.set(false)
  })
  return (
    <If when={props.enabled && manual() && !dismissed()}>
      <Modal title="音乐已就绪" onClose={close}>
        <View style={{ gap: 24 }}>
          <Text style={[font(), { color: muted }]}>浏览器需要你点击一次，才能开始播放音乐。</Text>
          <View style={{ flexDirection: 'row', justifyContent: 'flex-end', gap: 12 }}>
            <TextButton title="稍后播放" variant="secondary" onPress={close} />
            <TextButton
              title="开始播放"
              onPress={() => void playerStore.onCanPlay()}
            />
          </View>
        </View>
      </Modal>
    </If>
  )
}
