import type { User } from '@/types'
import MD5 from 'crypto-js/md5'
/**
 * 生成 Gravatar 头像 URL
 * @param emailOrHash 邮箱地址或MD5哈希值
 * @param size 头像尺寸，默认 200
 * @param defaultImage 默认头像类型，默认 'mp'
 * @returns Gravatar URL
 */
export function generateGravatarUrl(
  emailOrHash: string,
  size: number = 200,
  defaultImage: string = 'mp',
): string {
  // 判断是否为32位MD5哈希（仅包含十六进制字符）
  const md5Regex = /^[a-f0-9]{32}$/i
  let hash = ''
  if (md5Regex.test(emailOrHash)) {
    hash = emailOrHash
  } else {
    const normalizedEmail = emailOrHash.toLowerCase().trim()
    hash = MD5(normalizedEmail).toString()
  }
  return `https://www.gravatar.com/avatar/${hash}?s=${size}&d=${defaultImage}`
}
/**
 * 处理用户对象，解析用户名并设置 Gravatar 头像
 * @param user 用户对象
 * @returns 处理后的用户对象
 */
export function processUser(user: User): User {
  let avatar = user.avatar
  if (user.emailHash) {
    avatar = generateGravatarUrl(user.emailHash)
  } else if (user.email) {
    avatar = generateGravatarUrl(user.email)
  } else if (!avatar) {
    avatar = `https://api.dicebear.com/7.x/initials/svg?seed=${encodeURIComponent(user.name || '匿名用户')}`
  }
  return { ...user, avatar }
}

/**
 * 批量处理用户列表
 * @param users 用户列表
 * @returns 处理后的用户列表
 */
export function processUsers(users: User[]): User[] {
  return users.map(processUser)
}

/**
 * 将协议用户 { name, emailHash } 适配为 UI 用户模型，
 * 头像统一交给 processUser 生成
 */
export function adaptProtocolUser(user: { name: string, emailHash: string }): User {
  return processUser({ name: user.name || '未知用户', emailHash: user.emailHash, avatar: '' })
}

/**
 * 保存房间密码到本地存储
 * @param roomId 房间ID
 * @param password 房间密码
 */
export function saveRoomPassword(roomId: string, password: string): void {
  localStorage.setItem(`alisten_room_password_${roomId}`, password)
}

/**
 * 从本地存储获取房间密码
 * @param roomId 房间ID
 * @returns 保存的密码，如果没有则返回 null
 */
export function getSavedRoomPassword(roomId: string): string | null {
  return localStorage.getItem(`alisten_room_password_${roomId}`)
}

/**
 * 清除保存的房间密码
 * @param roomId 房间ID
 */
export function clearRoomPassword(roomId: string): void {
  localStorage.removeItem(`alisten_room_password_${roomId}`)
}

/**
 * 保存上次进入的房间ID
 * @param roomId 房间ID
 */
export function saveLastJoinedRoom(roomId: string): void {
  localStorage.setItem('alisten_last_joined_room', roomId)
}

/**
 * 从本地存储获取上次进入的房间ID
 * @returns 保存的房间ID，如果没有则返回 null
 */
export function getLastJoinedRoom(): string | null {
  return localStorage.getItem('alisten_last_joined_room')
}

/**
 * 清除保存的上次进入房间记录
 */
export function clearLastJoinedRoom(): void {
  localStorage.removeItem('alisten_last_joined_room')
}

// 获取默认头像
export function getDefaultAvatar(seed?: string | number): string {
  const randomSeed = seed || Date.now()
  return `https://picsum.photos/200/200?random=${randomSeed}`
}
