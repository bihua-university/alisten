import { parseWebStream } from 'music-metadata'

const MAX_METADATA_BYTES = 1024 * 1024
const METADATA_TIMEOUT_MS = 5000

/** dB 转为线性振幅比例；0 dB 是有效值，缺失或无效时返回 null。 */
export function gainToCoefficient(gainDb?: number): number | null {
  if (gainDb === undefined || !Number.isFinite(gainDb))
    return null
  const coefficient = 10 ** (gainDb / 20)
  return Number.isFinite(coefficient) && coefficient > 0 ? coefficient : null
}

/** 只读取文件头的标签；跨域限制、缺失标签或解析失败时返回 null。 */
export async function readReplayGain(url: string, signal: AbortSignal): Promise<number | null> {
  const controller = new AbortController()
  const abort = () => controller.abort()
  signal.addEventListener('abort', abort, { once: true })
  const timeout = setTimeout(abort, METADATA_TIMEOUT_MS)
  let coefficient: number | null = null
  try {
    if (signal.aborted || controller.signal.aborted)
      return null
    const response = await fetch(url, {
      signal: controller.signal,
      headers: { Range: `bytes=0-${MAX_METADATA_BYTES - 1}` },
    })
    if (!response.ok || !response.body)
      return null

    // 即使 CDN 忽略 Range，也不为读取标签下载整首歌曲。
    let remaining = MAX_METADATA_BYTES
    const stream = response.body.pipeThrough(new TransformStream<Uint8Array, Uint8Array>({
      transform(chunk, output) {
        output.enqueue(chunk.subarray(0, remaining))
        remaining -= chunk.byteLength
        if (remaining <= 0)
          output.terminate()
      },
    }))
    await parseWebStream(stream, { mimeType: response.headers.get('Content-Type') || undefined }, {
      duration: false,
      skipCovers: true,
      skipPostHeaders: true,
      observer: ({ metadata }) => {
        const gainDb = metadata.common.replaygain_track_gain?.dB
        // 标签使用 dB；HTMLAudioElement.volume 使用振幅比例。
        const gain = gainToCoefficient(gainDb)
        if (gain !== null)
          coefficient = gain
      },
    })
  } catch {
    // 有限文件头可能在音频帧中截断，保留此前已经读到的标签。
  } finally {
    clearTimeout(timeout)
    signal.removeEventListener('abort', abort)
    controller.abort()
  }
  return signal.aborted ? null : coefficient
}
