import { Scroll, Text, View } from 'nsh'
import { useStore } from '@/hooks/use-store'
import { playerStore } from '@/stores/player-store'
import { websocketStore } from '@/stores/websocket-store'
import { accent, brand, compact, font, foreground, muted } from '@/theme'
import { SongArtwork } from './song-artwork'
import { SongList } from './song-list'
import { EmptyState, TextButton } from './ui'

export default function QueuePage(props: {
  onSearch: () => void
  compact?: boolean
}) {
  const songs = useStore(playerStore, state => state.playlist)
  const current = useStore(playerStore, state => state.currentSong)
  const connected = useStore(websocketStore, state => state.connectionStatus === 'connected')
  return (
    <Scroll accessibilityLabel="待播队列" scrollbar={false} style={{ flex: 1, minHeight: 0 }} contentStyle={{ padding: props.compact ? 16 : 24, paddingHorizontal: props.compact ? 16 : compact() ? 16 : 32, flexGrow: 1 }}>
      <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12, marginBottom: 16 }}>
        <View style={{ flexDirection: 'row', alignItems: 'baseline', gap: 6 }}>
          <Text style={[font(props.compact ? 0 : 14), { color: foreground, fontWeight: '600' }]}>播放列表</Text>
          <Text style={[font(-2), { color: muted }]}>{songs().length}</Text>
        </View>
        <TextButton title="搜索" variant="ghost" disabled={!connected()} onPress={props.onSearch} />
      </View>
      <If when={current()}>
        <View style={{ flexDirection: 'row', gap: 12, alignItems: 'center', marginBottom: 20, padding: 12, backgroundColor: accent, borderRadius: 12 }}>
          <SongArtwork song={current()!} size={48} />
          <View style={{ flex: 1, minWidth: 0, gap: 3 }}>
            <Text style={[font(-4), { color: brand }]}>当前歌曲</Text>
            <Text style={[font(), { color: foreground, fontWeight: '600', numberOfLines: 1 }]}>{current()!.title}</Text>
            <Text style={[font(-2), { color: muted, numberOfLines: 1 }]}>{current()!.artist}</Text>
          </View>
        </View>
      </If>
      <If
        when={songs().length}
        else={<EmptyState icon="ListMusic" title="队列还是空的" action={<TextButton title="搜索音乐" variant="secondary" disabled={!connected()} onPress={props.onSearch} />}>每个人都可以点歌，一起决定下一首。</EmptyState>}
      >
        <SongList songs={songs()} queue compact={props.compact} disabled={!connected()} />
      </If>
    </Scroll>
  )
}
