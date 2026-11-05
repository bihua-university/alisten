import type { RoomInfo } from '@/types'
import type { MusicSearchResult, PickRequest, PickResult, PlayableSong, PlaylistSearchResult, SearchRequest } from '@/types/livehouse-protocol'
import { roomStore } from '@/stores/room-store'
import { getAppConfig } from './config'
import { applyEnvironment } from './environment'

// 创建房间的接口类型
export interface CreateRoomRequest {
  name: string
  desc?: string
  needPwd: boolean
  password?: string
}

export interface CreateRoomResponse {
  houseId: string
}

/**
 * 创建房间
 * @param roomData 房间数据
 * @returns Promise<CreateRoomResponse>
 */
export async function createRoom(roomData: CreateRoomRequest): Promise<CreateRoomResponse> {
  const config = getAppConfig()
  const url = applyEnvironment(new URL('/api/house/add', config.api.baseUrl))

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
  const url = applyEnvironment(new URL('/api/house/search', config.api.baseUrl))

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

function roomCredentials() {
  return {
    houseId: roomStore.state.id,
    password: roomStore.getCurrentPassword() ?? '',
  }
}

async function postMusic<T>(path: string, data: object): Promise<T> {
  const url = applyEnvironment(new URL(path, getAppConfig().api.baseUrl))
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...roomCredentials(), ...data }),
  })
  const body = await response.json()
  if (!response.ok)
    throw new Error(body?.error || `HTTP ${response.status}`)
  return body as T
}

export function searchMusic(request: SearchRequest): Promise<MusicSearchResult> {
  return postMusic<MusicSearchResult>('/api/music/search', request)
}

export function searchPlaylists(request: SearchRequest): Promise<PlaylistSearchResult> {
  return postMusic<PlaylistSearchResult>('/api/music/searchsonglist', request)
}

export function pickMusic(request: PickRequest): Promise<PickResult> {
  return postMusic<PickResult>('/api/music/pick', request)
}

export function recommendMusic(id?: string): Promise<MusicSearchResult> {
  return postMusic<MusicSearchResult>('/api/music/recommend', { id })
}

export function resolveMusic(source: PlayableSong['source'], id: string): Promise<PlayableSong> {
  return postMusic<PlayableSong>('/api/music/resolve', { source, id })
}
