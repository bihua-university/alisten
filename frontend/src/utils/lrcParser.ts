import type { LyricLine } from '@/types'

/**
 * LRC 歌词解析器
 * 支持标准 LRC 格式的歌词文件解析
 */

export interface ParsedLyrics {
  lyrics: LyricLine[]
}

/**
 * 解析时间标签，格式: [mm:ss.xx] 或 [mm:ss]
 * @param timeStr 时间字符串，如 "02:35.50"
 * @returns 时间（秒）
 */
export function parseTimeTag(timeStr: string): number {
  const timeRegex = /^(\d{1,2}):(\d{2})(?:\.(\d{1,3}))?$/
  const match = timeStr.match(timeRegex)

  if (!match) {
    return 0
  }

  const minutes = Number.parseInt(match[1], 10)
  const seconds = Number.parseInt(match[2], 10)
  const milliseconds = match[3] ? Number.parseInt(match[3].padEnd(3, '0'), 10) : 0

  return minutes * 60 + seconds + milliseconds / 1000
}

/**
 * 解析 LRC 格式歌词
 * @param lrcContent LRC 格式的歌词内容
 * @returns 解析后的歌词对象
 */
export function parseLyrics(lrcContent: string): ParsedLyrics {
  const lines = lrcContent.split('\n')
  const lyrics: LyricLine[] = []

  // 用于存储带有多个时间标签的歌词行
  const pendingLyrics: { time: number, text: string }[] = []

  for (const line of lines) {
    const trimmedLine = line.trim()
    if (!trimmedLine)
      continue

    // 匹配时间标签和歌词 [mm:ss.xx]歌词内容
    const timeTagRegex = /\[(\d{1,2}:\d{2}(?:\.\d{1,3})?)\]/g
    const timeTags: string[] = []

    // 使用 matchAll 提取所有时间标签
    const matches = Array.from(trimmedLine.matchAll(timeTagRegex))
    for (const match of matches) {
      timeTags.push(match[1])
    }

    if (timeTags.length > 0) {
      // 提取歌词文本（去除所有时间标签）
      const lyricText = trimmedLine.replace(/\[\d{1,2}:\d{2}(?:\.\d{1,3})?\]/g, '').trim()

      // 为每个时间标签创建歌词行
      for (const timeTag of timeTags) {
        const time = parseTimeTag(timeTag)
        pendingLyrics.push({ time, text: lyricText })
      }
    }
  }

  // 应用偏移量并排序
  pendingLyrics.forEach((lyric) => {
    lyrics.push({
      time: Math.max(0, lyric.time),
      text: lyric.text,
    })
  })

  // 按时间排序
  lyrics.sort((a, b) => a.time - b.time)

  return { lyrics }
}

/**
 * 验证 LRC 格式
 * @param content 要验证的内容
 * @returns 是否为有效的 LRC 格式
 */
export function isValidLrc(content: string): boolean {
  if (!content.trim())
    return false

  const lines = content.split('\n')
  let hasTimeTag = false

  for (const line of lines) {
    const trimmedLine = line.trim()
    if (!trimmedLine)
      continue

    // 检查是否包含时间标签
    if (/\[\d{1,2}:\d{2}(?:\.\d{1,3})?\]/.test(trimmedLine)) {
      hasTimeTag = true
      break
    }
  }

  return hasTimeTag
}
