import type { Song } from '@/types'
import type { Source } from '@/types/livehouse-protocol'
import { Text, View } from 'nsh'
import { playerStore } from '@/stores/player-store'
import { websocketStore } from '@/stores/websocket-store'
import { brand, compact, font, foreground, muted, surfaceHover } from '@/theme'
import { formatTime } from '@/utils/time'
import { SongArtwork } from './song-artwork'
import { AsyncIconButton } from './ui'

export function SongRow(props: {
  song: Song
  index: number
  queue?: boolean
  disabled?: boolean
  compact?: boolean
}) {
  const mobile = () => compact() || props.compact
  return (
    <View hoverStyle={{ backgroundColor: surfaceHover }} style={{ minHeight: 68, flexDirection: 'row', alignItems: 'center', gap: mobile() ? 8 : 12, paddingHorizontal: 8, paddingVertical: 10, borderRadius: 8 }}>
      <If when={!mobile()}>
        <Text style={[font(-2), { width: 22, color: muted }]}>{String(props.index + 1).padStart(2, '0')}</Text>
      </If>
      <SongArtwork song={props.song} />
      <View style={{ flex: 1, minWidth: 0, gap: 3 }}>
        <Text style={[font(), { color: foreground, fontWeight: '600', numberOfLines: 1 }]}>{props.song.title}</Text>
        <Text style={[font(-2), { color: muted, numberOfLines: 1 }]}>{`${props.song.artist}${props.song.requestedBy ? ` · ${props.song.requestedBy.name} 点播` : ''}`}</Text>
      </View>
      <If when={!mobile()}>
        <Text style={[font(-2), { width: 150, maxWidth: '22%', color: muted, numberOfLines: 1 }]}>{props.song.album}</Text>
        <Text style={[font(-2), { color: muted, fontVariant: 'tabular-nums' }]}>{formatTime(props.song.duration / 1000)}</Text>
      </If>
      <If
        when={props.queue}
        else={(
          <AsyncIconButton
            icon="CirclePlus"
            label={`点播 ${props.song.title}`}
            color={brand}
            disabled={props.disabled}
            onAction={() => playerStore.pickMusic(props.song)}
          />
        )}
      >
        <View style={{ flexShrink: 0, flexDirection: 'row', alignItems: 'center' }}>
          <AsyncIconButton
            icon="Heart"
            label={`赞 ${props.song.title}`}
            color={brand}
            disabled={props.disabled || !props.song.id}
            onAction={() => websocketStore.sendSongLike(props.song.id!, (props.song.source || 'netease') as Source)}
          />
          <AsyncIconButton
            icon="Trash2"
            label={`移除 ${props.song.title}`}
            color={brand}
            disabled={props.disabled || !props.song.id}
            onAction={() => websocketStore.sendDeleteSong(props.song.id!, (props.song.source || 'netease') as Source)}
          />
        </View>
      </If>
    </View>
  )
}

export function SongList(props: {
  songs: Song[]
  queue?: boolean
  disabled?: boolean
  compact?: boolean
}) {
  return (
    <View>
      <Each of={props.songs} key={song => `${song.source}:${song.id || song.title}:${song.artist}`}>
        {(song, index) => <SongRow song={song()} index={index()} queue={props.queue} disabled={props.disabled} compact={props.compact} />}
      </Each>
    </View>
  )
}
