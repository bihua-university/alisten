import { CheckCircle2, CircleAlert, Info, TriangleAlert, X } from 'lucide-react'
import { useStore } from '@/hooks/use-store'
import { notificationStore } from '@/stores/notification-store'
import { playerStore } from '@/stores/player-store'
import { pwaStore } from '@/stores/pwa-store'
import { IconButton, Modal } from './ui'

const notificationIcons = {
  success: CheckCircle2,
  error: CircleAlert,
  warning: TriangleAlert,
  info: Info,
}

export function Notifications() {
  const notifications = useStore(notificationStore, s => s.notifications)
  return (
    <div className="notifications" aria-label="通知">
      {notifications
        .filter(n => n.show)
        .map((item) => {
          const Icon = notificationIcons[item.type || 'info']
          return (
            <div
              key={item.id}
              className={`toast toast-${item.type || 'info'}`}
              role={item.type === 'error' ? 'alert' : 'status'}
            >
              <Icon size={20} aria-hidden="true" />
              <p className="text-ui-base">{item.message}</p>
              <IconButton
                icon={X}
                label="关闭通知"
                onClick={() => notificationStore.hide(item.id)}
              />
            </div>
          )
        })}
    </div>
  )
}

export function SystemPrompts() {
  const manual = useStore(playerStore, s => s.needManualStartPlay)
  const update = useStore(pwaStore, s => s.showUpdateModal)
  const closeManual = () => playerStore.clearManualStartPlay()
  if (manual) {
    return (
      <Modal title="音乐已就绪" onClose={closeManual}>
        <p className="text-ui-base muted">
          浏览器需要你点击一次，才能开始播放音乐。
        </p>
        <div className="dialog-actions">
          <button
            type="button"
            className="button-secondary"
            onClick={closeManual}
          >
            稍后播放
          </button>
          <button
            type="button"
            className="button-primary"
            onClick={async () => {
              closeManual()
              await playerStore.onCanPlay()
            }}
          >
            开始播放
          </button>
        </div>
      </Modal>
    )
  }
  if (update) {
    return (
      <Modal
        title="新版本已就绪"
        onClose={() => pwaStore.handleDismissUpdate()}
      >
        <p className="text-ui-base muted">更新后将重新加载应用并连接房间。</p>
        <div className="dialog-actions">
          <button
            type="button"
            className="button-secondary"
            onClick={() => pwaStore.handleDismissUpdate()}
          >
            稍后再说
          </button>
          <button
            type="button"
            className="button-primary"
            onClick={() => pwaStore.handleUpdateApp()}
          >
            立即更新
          </button>
        </div>
      </Modal>
    )
  }
  return null
}
