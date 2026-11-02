import type { ChatMessage, User } from '@/types'
import type { MembersData } from '@/types/ws-protocol'
import { adaptProtocolUser, processUser } from '@/utils/user'
import { notificationStore } from './notification-store'
import { Store } from './store-base'
import { websocketStore } from './websocket-store'

interface ChatState {
  chatMessages: ChatMessage[]
  onlineUsers: User[]
}

class ChatStore extends Store<ChatState> {
  constructor() {
    super({ chatMessages: [], onlineUsers: [] })

    websocketStore.registerEventHandler('chat.message', (data) => {
      if (!data.content)
        return
      const msg: ChatMessage = {
        content: data.content,
        timestamp: data.sentAtMs || Date.now(),
        user: adaptProtocolUser(data.user),
      }
      this.setState({ chatMessages: [...this.state.chatMessages, msg] })
    })

    websocketStore.registerEventHandler('members.updated', (data) => {
      this.applyMembers(data)
    })
  }

  /** members.get 响应与 members.updated 事件共用 */
  private applyMembers(data: MembersData) {
    this.setState({ onlineUsers: data.users.map(adaptProtocolUser) })
  }

  get chatMessages(): ChatMessage[] {
    return this.state.chatMessages.map(message => ({
      ...message,
      user: processUser(message.user),
    }))
  }

  get onlineUsers(): User[] {
    return this.state.onlineUsers
  }

  addMessage(message: ChatMessage) {
    this.setState({ chatMessages: [...this.state.chatMessages, message] })
  }

  clearMessages() {
    this.setState({ chatMessages: [] })
  }

  updateOnlineUsers(users: User[]) {
    this.setState({ onlineUsers: [...users] })
  }

  reset() {
    this.setState({ chatMessages: [], onlineUsers: [] })
  }

  sendMessage(content: string) {
    if (content.trim()) {
      websocketStore.sendChatMessage(content)
    }
  }

  async refreshOnlineUsers() {
    try {
      const data = await websocketStore.request('members.get', {})
      this.applyMembers(data)
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : '获取在线用户失败')
    }
  }
}

export const chatStore = new ChatStore()
