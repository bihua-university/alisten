import { createAnimatedValue, effect, signal, Text, View } from 'nsh'
import { Slider } from 'nsh/base'
import { useStore } from '@/hooks/use-store'
import { playerStore } from '@/stores/player-store'
import { websocketStore } from '@/stores/websocket-store'
import { brand, compact, font, foreground, muted } from '@/theme'
import { lifecycle } from '@/utils/lifecycle'
import { formatTime } from '@/utils/time'
import { Artwork, IconButton } from './ui'

export function PlayerBar(props: {
  onLyrics: () => void
  onQueue: () => void
  onChat: () => void
  canOpenPanels: boolean
  chatOpen: boolean
  queueOpen: boolean
}) {
  const song = useStore(playerStore, state => state.currentSong)
  const volume = useStore(playerStore, state => state.volume)
  const isMuted = useStore(playerStore, state => state.isMuted)
  const manual = useStore(playerStore, state => state.needManualStartPlay)
  const connected = useStore(websocketStore, state => state.connectionStatus === 'connected')
  const skipping = signal(false)
  const mobile = compact
  return (
    <View
      accessibilityLabel="播放器"
      style={{
        position: 'relative',
        height: mobile() ? undefined : 104,
        minHeight: mobile() ? 68 : undefined,
        flexDirection: 'row',
        alignItems: 'center',
        gap: mobile() ? 12 : 16,
        paddingHorizontal: mobile() ? 8 : 16,
        paddingVertical: mobile() ? 6 : 12,
        marginHorizontal: mobile() ? 8 : 0,
        marginTop: mobile() ? 8 : 0,
        marginBottom: mobile() ? 4 : 0,
        borderRadius: mobile() ? 12 : 0,
        overflow: 'hidden',
        backgroundColor: mobile() ? song() ? '#203d43' : '#24282a' : '#000000',
        flexShrink: 0,
      }}
    >
      <View
        role="button"
        class="cursor-pointer"
        accessibilityLabel="查看专辑封面与歌词"
        onPress={props.onLyrics}
        style={{ width: mobile() ? 56 : 64, height: mobile() ? 56 : 64, flexShrink: 0, borderRadius: mobile() ? 6 : 12 }}
      >
        <Artwork src={song()?.cover} title={song()?.title || '暂无歌曲'} size={mobile() ? 56 : 64} radius={mobile() ? 6 : 12} />
      </View>
      <View style={{ flex: 1, minWidth: 0, position: 'relative' }}>
        <View style={{ paddingRight: mobile() ? 0 : 112 }}>
          <Text style={[font(4), { color: foreground, fontWeight: '600', numberOfLines: 1 }]}>{song()?.title || '暂无歌曲'}</Text>
          <Text style={[font(-1), { color: mobile() ? '#c1d3d5' : muted, marginTop: mobile() ? 5 : 3, numberOfLines: 1 }]}>{song()?.artist || (props.canOpenPanels ? '等待下一首歌曲' : '加入房间，一起听歌')}</Text>
        </View>
        <If when={!mobile()}><PlaybackProgress mobile={false} /></If>
      </View>
      <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: mobile() ? 0 : 8, paddingLeft: mobile() ? 0 : 12, width: mobile() ? manual() ? 88 : 44 : undefined, flexShrink: 0 }}>
        <If when={manual()}>
          <IconButton icon="Play" label="开始播放" color={brand} onPress={() => void playerStore.onCanPlay()} />
        </If>
        <IconButton
          icon="SkipForward"
          label="投票切歌"
          color={foreground}
          size={mobile() ? 18 : 20}
          style={{ borderRadius: mobile() ? 0 : 8, borderWidth: 0, backgroundColor: 'transparent' }}
          disabled={!song() || !connected() || skipping()}
          onPress={async () => {
            skipping.set(true)
            await playerStore.skipSong()
            skipping.set(false)
          }}
        />
        <If when={!mobile()}>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 4 }}>
            <IconButton icon={isMuted() ? 'VolumeX' : 'Volume2'} label={isMuted() ? '取消静音' : '静音'} color={foreground} onPress={() => playerStore.toggleMute()} />
            <Slider
              value={isMuted() ? 0 : volume()}
              min={0}
              max={100}
              onChange={({ value }) => {
                playerStore.setVolume(value as number)
                if (isMuted() && (value as number) > 0)
                  playerStore.toggleMute()
              }}
              accessibilityLabel="音量"
              style={{ width: compact() ? 60 : 90, marginRight: 16, flexDirection: 'row', alignItems: 'center' }}
              trackStyle={{ height: 18, flex: 1, justifyContent: 'center' }}
              indicatorStyle={{ height: 3, borderRadius: 4, backgroundColor: muted }}
              rangeStyle={{ backgroundColor: foreground }}
              thumbStyle={{ width: 12, height: 12, borderRadius: 999, backgroundColor: foreground }}
            />
            <IconButton icon="MessageCircle" label="房间聊天" selected={props.chatOpen} color={props.chatOpen ? brand : muted} disabled={!props.canOpenPanels} onPress={props.onChat} />
            <IconButton icon="ListMusic" label="待播队列" selected={props.queueOpen} color={props.queueOpen ? brand : muted} disabled={!props.canOpenPanels} onPress={props.onQueue} />
          </View>
        </If>
      </View>
      <If when={mobile()}><PlaybackProgress mobile /></If>
    </View>
  )
}

function PlaybackProgress(props: { mobile: boolean }) {
  const song = useStore(playerStore, state => state.currentSong)
  const playing = useStore(playerStore, state => state.isPlaying)
  const buffering = useStore(playerStore, state => state.isBuffering)
  const positionRevision = useStore(playerStore, state => state.positionRevision)
  const ratio = createAnimatedValue(0)
  const label = signal('0:00 / 0:00')
  const elapsed = signal(0)
  effect(() => {
    positionRevision()
    const duration = (song()?.duration || 0) / 1000
    const position = Math.max(0, Math.min(duration, playerStore.getCurrentTime()))
    const active = playing() && !buffering() && lifecycle() !== 'background'
    let lastFrame = Number.NEGATIVE_INFINITY
    let lastSecond = -1
    ratio.set(duration ? position / duration : 0)
    elapsed.set(Math.floor(position))
    if (!props.mobile)
      label.set(`${formatTime(position)} / ${formatTime(duration)}`)
    if (!active)
      ratio.stop()
    const update = (time: number) => {
      if (!active)
        return
      const now = performance.now()
      if (now - lastFrame < 1000 / 30)
        return
      lastFrame = now
      const clamped = Math.max(0, Math.min(duration, time))
      ratio.set(duration ? clamped / duration : 0)
      const second = Math.floor(clamped)
      if (second !== lastSecond) {
        lastSecond = second
        elapsed.set(second)
        if (!props.mobile)
          label.set(`${formatTime(clamped)} / ${formatTime(duration)}`)
      }
    }
    update(position)
    playerStore.registerProgressCallback(update)
    return () => {
      playerStore.unregisterProgressCallback(update)
      ratio.stop()
    }
  })
  return (
    <>
      <If when={!props.mobile}>
        <Text style={[font(-3), { position: 'absolute', right: 0, top: 34, color: muted, textAlign: 'right', fontVariant: 'tabular-nums' }]}>{label()}</Text>
      </If>
      <View
        accessibilityLabel="播放进度"
        role="progressbar"
        accessibilityValue={{ min: 0, max: Math.floor((song()?.duration || 0) / 1000), now: elapsed() }}
        style={{ position: props.mobile ? 'absolute' : 'relative', left: 0, right: 0, bottom: props.mobile ? 0 : undefined, height: 3, marginTop: props.mobile ? 0 : 10, borderRadius: props.mobile ? 0 : 4, overflow: 'hidden', backgroundColor: props.mobile ? '#ffffff26' : '#585860' }}
      >
        <View style={{ height: '100%', backgroundColor: foreground, transformOrigin: 'left', scale: [ratio, 1] }} />
      </View>
    </>
  )
}
