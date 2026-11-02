import type { RoomInfo } from '@/types'
import { Store } from './store-base'

const initialState: RoomInfo = {
  id: '733dbb38-31d0-419c-9019-5c12777246c8',
  name: '听歌房',
  description: '欢迎来到听歌房！',
  population: 0,
  needPwd: true,
  ultimate: false,
}

let currentPassword: string | undefined

class RoomStore extends Store<RoomInfo> {
  updateRoomInfo(newInfo: Partial<RoomInfo>) {
    this.setState({ ...this.state, ...newInfo })
  }

  setRoomId(id: string) {
    this.setState({ ...this.state, id })
  }

  setRoomName(name: string) {
    this.setState({ ...this.state, name })
  }

  resetRoomInfo() {
    this.setState({ ...initialState })
  }

  setCurrentPassword(password?: string) {
    currentPassword = password
  }

  getCurrentPassword(): string | undefined {
    return currentPassword
  }

  clearCurrentPassword() {
    currentPassword = undefined
  }
}

export const roomStore = new RoomStore(initialState)
