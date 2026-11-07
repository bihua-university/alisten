import type { ScrollRef } from 'nsh'
import type { LyricContainer } from '@/stores/lyrics-store'
import { batch, cleanup, computed, createAnimatedValue, Easing, effect, platform, Scroll, signal, Text, untracked, View } from 'nsh'
import { useStore } from '@/hooks/use-store'
import { lyricsStore } from '@/stores/lyrics-store'
import { compact, font, foreground, motionDuration, muted } from '@/theme'
import { lifecycle } from '@/utils/lifecycle'
import { EmptyState } from './ui'

export function Lyrics() {
  const lyrics = useStore(lyricsStore, state => state.currentLyrics)
  const currentIndex = useStore(lyricsStore, state => state.currentLyricIndex)
  const fromIndex = signal(currentIndex())
  const toIndex = signal(currentIndex())
  const progress = createAnimatedValue(1)
  const viewportHeight = signal(0)
  const frames = new Map<number, { text: string, height: number }>()
  let layoutRaf: number | undefined
  let viewportWidth = 0
  let contentWidth = 0
  let contentHeight = 0
  let scroller: ScrollRef | undefined
  let initial = true
  let needsInitialAlignment = true
  const mobile = compact
  const styleTransitions = platform.os === 'web'

  effect(() => {
    const next = currentIndex()
    // Web uses CSS. Native platforms play this one animation on the UI thread.
    if (styleTransitions) {
      toIndex.set(next)
      return
    }
    const previous = untracked(toIndex)
    if (initial) {
      fromIndex.set(next)
      toIndex.set(next)
      progress.set(1)
      initial = false
      return
    }
    batch(() => {
      fromIndex.set(previous)
      toIndex.set(next)
      progress.set(0)
    })
    const duration = lyricMotionDuration()
    if (duration === 0) {
      progress.set(1)
    } else {
      void progress.timing(1, { duration, easing: Easing.out(Easing.cubic) })
    }
    return () => progress.stop()
  })

  const container: LyricContainer = {
    reset() {
      if (layoutRaf !== undefined) {
        cancelAnimationFrame(layoutRaf)
        layoutRaf = undefined
      }
      initial = true
      needsInitialAlignment = true
      progress.stop()
      batch(() => {
        fromIndex.set(-1)
        toIndex.set(-1)
        progress.set(1)
      })
      scroller?.scrollTo({ y: 0, animated: false })
    },
    scrollToCenter(index, smooth) {
      if (lifecycle() === 'background')
        return
      const lines = lyrics()
      if (!frames.size || !viewportHeight())
        return
      const positions = new Map<number, { top: number, height: number }>()
      let top = viewportHeight() / 2
      for (const [lineIndex, line] of lines.entries()) {
        if (!line.text)
          continue
        const frame = frames.get(lineIndex)
        // Only the prefix through the target is needed for its position.
        // Later rows may still be waiting for native layout callbacks.
        if (!frame || frame.text !== line.text)
          return
        positions.set(lineIndex, { top, height: frame.height })
        top += frame.height
        if (lineIndex >= index)
          break
      }
      let before: { top: number, height: number } | undefined
      let after: { top: number, height: number } | undefined
      for (const [lineIndex, frame] of positions) {
        if (lineIndex <= index)
          before = frame
        if (lineIndex >= index) {
          after = frame
          break
        }
      }
      before ??= after
      after ??= before
      if (!before || !after)
        return
      const center = (before.top + before.height / 2 + after.top + after.height / 2) / 2
      scroller?.scrollTo({ y: Math.max(0, center - viewportHeight() / 2), animated: smooth && !needsInitialAlignment })
      needsInitialAlignment = false
    },
  }
  const recenterAfterLayout = () => {
    if (layoutRaf !== undefined)
      cancelAnimationFrame(layoutRaf)
    layoutRaf = requestAnimationFrame(() => {
      layoutRaf = undefined
      container.scrollToCenter(currentIndex(), false)
    })
  }
  cleanup(() => {
    if (layoutRaf !== undefined)
      cancelAnimationFrame(layoutRaf)
  })
  effect(() => {
    lyricsStore.registerContainer(container)
    return () => lyricsStore.unregisterContainer(container)
  })
  effect(() => {
    viewportHeight()
    lifecycle()
    // Resizing does not emit new layout events for rows whose height stayed
    // the same. Retain those measurements and replace changed rows in onLayout.
    const lines = lyrics()
    for (const [index, frame] of frames) {
      if (lines[index]?.text !== frame.text)
        frames.delete(index)
    }
    recenterAfterLayout()
  })

  return (
    <View style={{ flex: 1, minHeight: 0, padding: mobile() ? 16 : 24, paddingHorizontal: mobile() ? 16 : 32 }}>
      <If
        when={lyrics().some(line => line.text)}
        else={<EmptyState icon="Music2" title="暂无歌词">当前歌曲没有可用歌词，静享音乐吧。</EmptyState>}
      >
        <View style={{ flex: 1, minHeight: 0, position: 'relative' }}>
          <Scroll
            ref={scroller}
            scrollbar={false}
            onLayout={({ bounds }) => {
              if (viewportWidth === bounds.size.width && viewportHeight() === bounds.size.height)
                return
              viewportWidth = bounds.size.width
              viewportHeight.set(bounds.size.height)
              recenterAfterLayout()
            }}
            style={{ flex: 1, minHeight: 0 }}
            contentStyle={{ flexGrow: 1 }}
          >
            <View
              onLayout={({ bounds }) => {
                if (contentWidth === bounds.size.width && contentHeight === bounds.size.height)
                  return
                contentWidth = bounds.size.width
                contentHeight = bounds.size.height
                recenterAfterLayout()
              }}
              style={{ flexGrow: 1, minWidth: 0 }}
            >
              <View style={{ height: viewportHeight() / 2 }} />
              {/* A new text with the same height may not emit onLayout when
                  an index-keyed native row is reused. Remount changed text so
                  every measurement pruned on song changes is reported again. */}
              <Each of={lyrics()} key={(line, index) => `${index}:${line.text}`}>
                {(line, index) => (
                  <If when={line().text}>
                    <LyricLine
                      text={line().text}
                      index={index()}
                      fromIndex={fromIndex}
                      toIndex={toIndex}
                      progress={progress}
                      onLayout={(height) => {
                        const previous = frames.get(index())
                        if (previous?.text === line().text && previous.height === height)
                          return
                        frames.set(index(), { text: line().text, height })
                        recenterAfterLayout()
                      }}
                    />
                  </If>
                )}
              </Each>
              <View style={{ height: viewportHeight() / 2 }} />
            </View>
          </Scroll>
          <View
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              height: '12%',
              backgroundImage: 'linear-gradient(to bottom, #121212, rgba(18,18,18,0))',
              pointerEvents: 'none',
              zIndex: 2,
            }}
          />
          <View
            style={{
              position: 'absolute',
              bottom: 0,
              left: 0,
              right: 0,
              height: '12%',
              backgroundImage: 'linear-gradient(to top, #121212, rgba(18,18,18,0))',
              pointerEvents: 'none',
              zIndex: 2,
            }}
          />
        </View>
      </If>
    </View>
  )
}

function LyricLine(props: {
  text: string
  index: number
  fromIndex: () => number
  toIndex: () => number
  progress: ReturnType<typeof createAnimatedValue>
  onLayout: (height: number) => void
}) {
  const styleTransitions = platform.os === 'web'
  const start = () => lyricLevel(props.text, props.index, props.fromIndex())
  const end = () => lyricLevel(props.text, props.index, props.toIndex())
  const scale = computed(() => styleTransitions || lyricMotionDuration() <= 160 ? levelScale(end()) : props.progress.interpolate({ inputRange: [0, 1], outputRange: [levelScale(start()), levelScale(end())] }))
  const translateY = computed(() => styleTransitions || lyricMotionDuration() <= 160 ? levelTranslate(end()) : props.progress.interpolate({ inputRange: [0, 1], outputRange: [levelTranslate(start()), levelTranslate(end())] }))
  const opacity = computed(() => styleTransitions ? levelOpacity(end()) : props.progress.interpolate({ inputRange: [0, 1], outputRange: [levelOpacity(start()), levelOpacity(end())] }))
  const color = computed(() => styleTransitions || lyricMotionDuration() <= 160 ? end() === 2 ? foreground : muted : props.progress.interpolate({ inputRange: [0, 1], outputRange: [start() === 2 ? foreground : muted, end() === 2 ? foreground : muted] }))
  return (
    <Text
      accessibilityState={{ selected: !!props.text && props.toIndex() === props.index }}
      onLayout={({ bounds }) => props.onLayout(bounds.size.height)}
      style={{
        maxWidth: 880,
        marginHorizontal: 'auto',
        paddingHorizontal: 32,
        paddingVertical: 8,
        fontSize: font(14).fontSize,
        lineHeight: (font(14).fontSize ?? 28) * 1.5,
        textAlign: 'center',
        color: color(),
        opacity: opacity(),
        fontWeight: '600',
        transformOrigin: 'center',
        translate: [0, translateY()],
        scale: scale(),
        transitionProperty: styleTransitions ? lyricMotionDuration() <= 160 ? 'opacity' : 'transform, opacity, color' : 'none',
        transitionDuration: styleTransitions ? lyricMotionDuration() : 0,
        transitionTimingFunction: 'cubic-bezier(.2, .8, .2, 1)',
      }}
    >
      {props.text}
    </Text>
  )
}

function lyricMotionDuration() {
  return lifecycle() === 'background' ? 0 : motionDuration(420)
}

function lyricLevel(text: string, index: number, current: number) {
  if (!text)
    return 0
  const distance = Math.abs(current - index)
  return distance === 0 ? 2 : distance === 1 ? 1 : 0
}

function levelScale(level: number) {
  return level === 2 ? 1.18 : level === 1 ? 0.8 : 0.7
}

function levelTranslate(level: number) {
  return level === 2 ? 0 : level === 1 ? 2 : 4
}

function levelOpacity(level: number) {
  return level === 2 ? 1 : level === 1 ? 0.9 : 0.78
}
