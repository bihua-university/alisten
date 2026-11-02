import { AlignLeft } from 'lucide-react'
import { memo, useEffect, useRef } from 'react'
import { useStore } from '@/hooks/use-store'
import { lyricsStore } from '@/stores/lyrics-store'
import { EmptyState } from './ui'

export const Lyrics = memo(() => {
  const lines = useStore(lyricsStore, s => s.currentLyrics)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const container = ref.current!
    const nodes = new Map(Array.from(container.querySelectorAll<HTMLElement>('[data-lyric-index]'), node => [Number(node.dataset.lyricIndex), node]))
    for (const node of nodes.values()) {
      node.classList.remove('active-lyric', 'near-lyric')
      node.removeAttribute('aria-current')
    }
    let active = -2
    let near: Element[] = []
    const update = () => {
      if (!lines.length)
        return
      const index = lyricsStore.state.currentLyricIndex
      if (index === active)
        return
      const previous = nodes.get(active)
      near.forEach(node => node.classList.remove('near-lyric'))
      near = []
      previous?.classList.remove('active-lyric')
      previous?.removeAttribute('aria-current')
      const current = nodes.get(index)
      if (current) {
        current.classList.add('active-lyric')
        current.setAttribute('aria-current', 'true')
        near = [current.previousElementSibling, current.nextElementSibling].filter((node): node is Element => !!node)
      } else {
        let before: HTMLElement | undefined
        for (const [lineIndex, node] of nodes) {
          if (lineIndex < index) {
            before = node
          } else {
            near.push(node)
            break
          }
        }
        if (before)
          near.push(before)
      }
      near.forEach(node => node.classList.add('near-lyric'))
      active = index
    }
    lyricsStore.registerContainer(container)
    update()
    // Initial position is immediate; subsequent line changes are scrolled
    // once by the store's rAF, alongside the CSS highlight transition.
    lyricsStore.syncScrollAllContainers(false)
    const resizeObserver = new ResizeObserver(() => lyricsStore.syncScrollAllContainers(false))
    resizeObserver.observe(container)
    lyricsStore.addEventListener('change', update)
    return () => {
      resizeObserver.disconnect()
      lyricsStore.removeEventListener('change', update)
      lyricsStore.unregisterContainer(container)
    }
  }, [lines])
  return (
    <section className="lyrics-panel" aria-label="同步歌词">
      <div className="lyrics-scroll" ref={ref}>
        {lines.some(line => line.text)
          ? (
              lines.map((line, index) => line.text
                ? (
                    <p
                      className="lyric-line text-ui-xl"
                      data-lyric-index={index}
                      key={`${line.time}:${line.text}`}
                    >
                      {line.text}
                    </p>
                  )
                : null)
            )
          : (
              <EmptyState icon={AlignLeft} title="歌词随音乐浮现">
                播放带有歌词的歌曲时，会在这里同步显示。
              </EmptyState>
            )}
      </div>
    </section>
  )
})
