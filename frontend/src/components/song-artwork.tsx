import type { Song } from '@/types'
import { View } from 'nsh'
import { Link } from 'nsh/base'
import { brand } from '@/theme'
import { Icon } from './icon'
import { Artwork } from './ui'

export function SongArtwork(props: { song?: Song, size?: number }) {
  const href = () => {
    try {
      const url = new URL(props.song?.webUrl || '')
      if (url.protocol === 'https:' || url.protocol === 'http:')
        return url.href
    } catch {}
    return undefined
  }
  const size = props.size ?? 42
  return (
    <If when={href()} else={<Artwork src={props.song?.cover} title={props.song?.title || '暂无歌曲'} size={size} />}>
      {url => (
        <Link
          href={url()}
          accessibilityLabel={`在音乐源打开 ${props.song?.title || '歌曲'}（新标签页）`}
          class="group cursor-pointer"
          style={{ position: 'relative', width: size, height: size, borderRadius: 8 }}
        >
          <Artwork src={props.song?.cover} title={props.song?.title || '暂无歌曲'} size={size} />
          <View class="opacity-0 group-hover:opacity-100 group-focus:opacity-100" style={{ position: 'absolute', top: 0, right: 0, bottom: 0, left: 0, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(0,0,0,0.55)', borderRadius: 8, pointerEvents: 'none' }}>
            <Icon name="ExternalLink" size={16} color={brand} />
          </View>
        </Link>
      )}
    </If>
  )
}
