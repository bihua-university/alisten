import { MessageCircle, Send, Users, X } from 'lucide-react'
import { memo, useEffect, useId, useRef, useState } from 'react'
import { useStore } from '@/hooks/use-store'
import { chatStore } from '@/stores/chat-store'
import { userSettingsStore } from '@/stores/user-settings-store'
import { websocketStore } from '@/stores/websocket-store'
import { Artwork, EmptyState, IconButton } from './ui'

export const ChatPanel = memo(({ onClose }: { onClose?: () => void }) => {
  const messages = useStore(chatStore, s => s.chatMessages)
  const users = useStore(chatStore, s => s.onlineUsers)
  const userName = useStore(userSettingsStore, s => s.userName)
  const connected = useStore(
    websocketStore,
    s => s.connectionStatus === 'connected',
  )
  const [draft, setDraft] = useState('')
  const [showUsers, setShowUsers] = useState(false)
  const [visibleCount, setVisibleCount] = useState(80)
  const [readCount, setReadCount] = useState(messages.length)
  const [atBottom, setAtBottom] = useState(true)
  const membersId = useId()
  const list = useRef<HTMLDivElement>(null)
  const members = useRef<HTMLDivElement>(null)
  const pinned = useRef(true)
  useEffect(() => {
    if (pinned.current)
      list.current?.scrollTo({ top: list.current.scrollHeight })
  }, [messages])
  useEffect(() => {
    if (!showUsers)
      return
    const dismiss = (e: PointerEvent) => {
      if (!members.current?.contains(e.target as Node))
        setShowUsers(false)
    }
    const escape = (e: KeyboardEvent) => {
      if (e.key === 'Escape')
        setShowUsers(false)
    }
    document.addEventListener('pointerdown', dismiss)
    document.addEventListener('keydown', escape)
    return () => {
      document.removeEventListener('pointerdown', dismiss)
      document.removeEventListener('keydown', escape)
    }
  }, [showUsers])
  const first = Math.max(0, messages.length - visibleCount)
  const hasNew = !atBottom && messages.length > readCount
  return (
    <section className="chat-panel" aria-label="房间聊天">
      <header className="chat-heading">
        <h2 className="text-ui-lg">房间聊天</h2>
        <div className="chat-heading-actions">
          <div className="member-popover-anchor" ref={members}>
            <button
              type="button"
              className="button-ghost chat-members-toggle text-ui-sm"
              aria-label={`在线成员，${users.length} 人`}
              aria-controls={membersId}
              aria-expanded={showUsers}
              aria-haspopup="dialog"
              onClick={() => setShowUsers(!showUsers)}
            >
              <Users size={16} aria-hidden="true" />
              {users.length}
              {' '}
              人
            </button>
            {showUsers && (
              <div className="member-popover" id={membersId} role="dialog" aria-label="在线成员">
                <div className="member-popover-head">
                  <strong className="text-ui-sm">在线成员</strong>
                  <span className="text-ui-xs muted">
                    {users.length}
                    {' '}
                    人
                  </span>
                </div>
                <div className="member-popover-list">
                  {users.map(user => (
                    <div
                      key={`${user.name}:${user.emailHash || user.avatar}`}
                      className="member text-ui-sm"
                    >
                      <Artwork src={user.avatar} title={user.name} />
                      <span className="truncate">{user.name}</span>
                    </div>
                  ))}
                  {!users.length && (
                    <p className="muted text-ui-sm">连接后显示在线成员</p>
                  )}
                </div>
              </div>
            )}
          </div>
          {onClose && <IconButton icon={X} label="关闭侧栏" onClick={onClose} />}
        </div>
      </header>
      <div
        className="chat-messages"
        ref={list}
        onScroll={() => {
          const el = list.current!
          pinned.current = el.scrollHeight - el.scrollTop - el.clientHeight < 60
          setAtBottom(pinned.current)
          if (pinned.current)
            setReadCount(messages.length)
        }}
      >
        {first > 0 && (
          <button
            type="button"
            className="button-ghost text-ui-sm"
            onClick={() => setVisibleCount(n => n + 80)}
          >
            查看更早的消息
          </button>
        )}
        {!messages.length && (
          <EmptyState icon={MessageCircle} title="好音乐，一起分享">
            打个招呼，或分享这首歌带给你的心情。
          </EmptyState>
        )}
        {messages.slice(first).map((message, i) => (
          <article
            className={`chat-message ${message.user.name === (userName || '匿名用户') ? 'self' : ''}`}
            key={`${message.timestamp}:${first + i}`}
          >
            <Artwork src={message.user.avatar} title={message.user.name} />
            <div className="message-content">
              <div className="message-meta text-ui-xs muted">
                <span>{message.user.name}</span>
                <time dateTime={new Date(message.timestamp).toISOString()}>
                  {new Date(message.timestamp).toLocaleTimeString('zh-CN', {
                    hour: '2-digit',
                    minute: '2-digit',
                  })}
                </time>
              </div>
              <p className="message-bubble text-ui-base">{message.content}</p>
            </div>
          </article>
        ))}
      </div>
      {hasNew && (
        <button
          type="button"
          className="new-messages text-ui-sm"
          onClick={() => {
            list.current?.scrollTo({ top: list.current.scrollHeight })
            pinned.current = true
            setReadCount(messages.length)
            setAtBottom(true)
          }}
        >
          查看新消息
        </button>
      )}
      <form
        className="chat-composer"
        onSubmit={(e) => {
          e.preventDefault()
          if (connected && websocketStore.sendChatMessage(draft))
            setDraft('')
        }}
      >
        <input
          type="text"
          aria-label="聊天消息"
          placeholder={connected ? '聊聊这首歌…' : '连接房间后开始聊天'}
          value={draft}
          onChange={e => setDraft(e.target.value)}
          maxLength={500}
          disabled={!connected}
        />
        <IconButton
          icon={Send}
          label="发送消息"
          type="submit"
          className="chat-send"
          disabled={!connected || !draft.trim()}
        />
      </form>
    </section>
  )
})
