import type { RoomInfo } from '@/types'
import { getAppConfig } from './config'

// 创建房间的接口类型
export interface CreateRoomRequest {
  name: string
  desc?: string
  needPwd: boolean
  password?: string
}

export interface CreateRoomResponse {
  code: string
  data: string // 房间ID
  message: string
}

/**
 * 创建房间
 * @param roomData 房间数据
 * @returns Promise<CreateRoomResponse>
 */
export async function createRoom(roomData: CreateRoomRequest): Promise<CreateRoomResponse> {
  const config = getAppConfig()
  const url = new URL('/house/add', config.api.baseUrl)

  try {
    const response = await fetch(url.toString(), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(roomData),
    })

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const data = await response.json()
    return data
  } catch (error) {
    console.error('创建房间失败:', error)
    throw error
  }
}

/**
 * 搜索房间列表
 * @param keyword 搜索关键词（可选）
 * @returns Promise<RoomSearchResponse>
 */
export async function searchRooms(keyword?: string, signal?: AbortSignal): Promise<RoomInfo[]> {
  const config = getAppConfig()
  const url = new URL('/house/search', config.api.baseUrl)

  if (keyword) {
    url.searchParams.append('keyword', keyword)
  }

  try {
    const response = await fetch(url.toString(), {
      method: 'GET',
      signal,
      headers: {
        'Content-Type': 'application/json',
      },
    })

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const data = await response.json()

    return data as RoomInfo[]
  } catch (error) {
    console.error('搜索房间失败:', error)
    throw error
  }
}
