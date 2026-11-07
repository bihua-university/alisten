import { computed, Scroll, signal, Text, View } from 'nsh'
import { Input, Switch } from 'nsh/base'
import { useStore } from '@/hooks/use-store'
import { historyStore } from '@/stores/history-store'
import { notificationStore } from '@/stores/notification-store'
import { playerStore } from '@/stores/player-store'
import { websocketStore } from '@/stores/websocket-store'
import { brand, compact, font, foreground, input, inputBorder, muted, subtle } from '@/theme'
import { Icon } from './icon'
import { SongArtwork } from './song-artwork'
import { AsyncIconButton, EmptyState, IconButton, Modal, TextButton } from './ui'

export default function HistoryPage(props: { compact?: boolean }) {
  const history = useStore(historyStore, state => state.playHistory)
  const recording = useStore(historyStore, state => state.isRecording)
  const connected = useStore(websocketStore, state => state.connectionStatus === 'connected')
  const query = signal('')
  const limit = signal(80)
  const confirm = signal(false)
  const filtered = computed(() => history().filter(item => `${item.song.title} ${item.song.artist} ${item.song.album}`.toLowerCase().includes(query().toLowerCase())))
  const rows = computed(() => {
    let lastDate = ''
    return filtered().slice(0, limit()).map((item) => {
      const date = new Date(item.playedAt).toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', year: 'numeric' })
      const heading = date !== lastDate
      lastDate = date
      return { item, date, heading }
    })
  })
  const mobile = compact
  return (
    <View style={{ flex: 1, minHeight: 0 }}>
      <Scroll scrollbar={false} style={{ flex: 1 }} contentStyle={{ flexGrow: 1, paddingHorizontal: props.compact ? 16 : mobile() ? 16 : 32, paddingVertical: props.compact ? 8 : mobile() ? 24 : 32 }}>
        <If when={!props.compact}>
          <View style={{ marginBottom: 24, gap: 8 }}>
            <Text style={[font(14), { color: foreground, fontWeight: '600' }]}>播放历史</Text>
            <Text style={[font(), { color: muted }]}>重温那些一起听过的旋律。</Text>
          </View>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 24 }}>
            <View style={{ flex: 1, minWidth: 0, flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 12, borderWidth: 1, borderColor: inputBorder, borderRadius: 8, backgroundColor: input }}>
              <Icon name="Search" size={18} />
              <Input
                value={query()}
                onChange={({ value }) => {
                  query.set(value)
                  limit.set(80)
                }}
                placeholder="搜索播放历史"
                accessibilityLabel="搜索播放历史"
                style={{ flex: 1, minWidth: 0, paddingVertical: 11, color: foreground, placeholderColor: subtle, fontSize: mobile() ? 16 : font().fontSize }}
              />
            </View>
            <IconButton icon="Download" label="导出播放历史" disabled={!history().length} onPress={() => historyStore.exportPlayHistory()} />
            <IconButton icon="Trash2" label="清空播放历史" disabled={!history().length} onPress={() => confirm.set(true)} />
          </View>
          <View style={{ flexDirection: mobile() ? 'column' : 'row', alignItems: mobile() ? 'flex-start' : 'center', justifyContent: 'space-between', gap: 12, marginBottom: 16 }}>
            <Text style={[font(-2), { color: muted }]}>{`${history().length} 首歌曲 · 保存在此设备`}</Text>
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
              <Switch
                checked={recording()}
                onChange={() => historyStore.togglePlayHistoryRecording()}
                accessibilityLabel="记录播放历史"
                trackStyle={{ width: 40, height: 24, padding: 2, borderRadius: 999, backgroundColor: recording() ? brand : '#3b3b3e' }}
                thumbStyle={{ width: 20, height: 20, borderRadius: 999, backgroundColor: foreground, translate: recording() ? [16, 0] : [0, 0] }}
              />
              <Text style={[font(-2), { color: foreground }]}>记录播放历史</Text>
            </View>
          </View>
        </If>
        <If
          when={filtered().length}
          else={<EmptyState icon="History" title={query() ? '没有找到相关记录' : '你的音乐足迹，从这里开始'}>{query() ? '换个关键词试试。' : '房间播放过的歌曲会保存在此设备，方便再次点播。'}</EmptyState>}
        >
          <Each of={rows()} key={row => `${row.item.song.source}:${row.item.song.id}:${row.item.playedAt}`}>
            {row => (
              <View>
                <If when={row().heading}>
                  <Text style={[font(-2), { color: muted, paddingHorizontal: 8, paddingTop: 16, paddingBottom: 8 }]}>{row().date}</Text>
                </If>
                <View style={{ minHeight: 68, flexDirection: 'row', alignItems: 'center', gap: 8, paddingHorizontal: 8, paddingVertical: 10, borderRadius: 8 }}>
                  <SongArtwork song={row().item.song} />
                  <View style={{ flex: 1, minWidth: 0, gap: 3 }}>
                    <Text style={[font(), { color: foreground, fontWeight: '600', numberOfLines: 1 }]}>{row().item.song.title}</Text>
                    <Text style={[font(-2), { color: muted, numberOfLines: 1 }]}>{`${row().item.song.artist} · ${row().item.song.album}`}</Text>
                  </View>
                  <If when={!props.compact && !mobile()}>
                    <Text style={[font(-2), { color: muted }]}>{new Date(row().item.playedAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</Text>
                  </If>
                  <AsyncIconButton icon="CirclePlus" label={`点播 ${row().item.song.title}`} color={brand} disabled={!connected()} onAction={() => playerStore.pickMusic(row().item.song)} />
                  <IconButton
                    icon="Trash2"
                    label={`删除 ${row().item.song.title} 的记录`}
                    color={brand}
                    onPress={() => {
                      historyStore.removeFromPlayHistory(history().indexOf(row().item))
                      notificationStore.success('已删除播放记录')
                    }}
                  />
                </View>
              </View>
            )}
          </Each>
          <If when={filtered().length > limit()}>
            <TextButton title="显示更多" variant="secondary" onPress={() => limit.update(value => value + 80)} style={{ alignSelf: 'center', marginTop: 24 }} />
          </If>
        </If>
      </Scroll>
      <If when={confirm()}>
        <Modal title="清空播放历史" onClose={() => confirm.set(false)}>
          <View style={{ gap: 24 }}>
            <Text style={[font(), { color: muted }]}>将清除此设备上的全部播放记录。</Text>
            <View style={{ flexDirection: mobile() ? 'column' : 'row', alignItems: mobile() ? 'stretch' : undefined, justifyContent: 'flex-end', gap: 12 }}>
              <TextButton title="取消" variant="secondary" onPress={() => confirm.set(false)} />
              <TextButton
                title="清空记录"
                variant="destructive"
                onPress={() => {
                  historyStore.clearPlayHistory()
                  confirm.set(false)
                }}
              />
            </View>
          </View>
        </Modal>
      </If>
    </View>
  )
}
