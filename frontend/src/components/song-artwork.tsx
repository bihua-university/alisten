import type { Song } from '@/types'
import { ExternalLink } from 'lucide-react'
import { Artwork } from './ui'

export function SongArtwork({ song }: { song: Song }) {
  let href: string | undefined
  try {
    const url = new URL(song.webUrl || '')
    if (url.protocol === 'https:' || url.protocol === 'http:')
      href = url.href
  } catch {
    // Songs without a valid source page keep their non-interactive artwork.
  }
  if (!href)
    return <Artwork src={song.cover} title={song.title} />
  return (
    <a
      className="song-artwork"
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`在音乐源打开 ${song.title}（新标签页）`}
      title="在音乐源打开（新标签页）"
    >
      <Artwork src={song.cover} title={song.title} />
      <ExternalLink size={16} strokeWidth={1.7} className="song-artwork-icon" aria-hidden="true" />
    </a>
  )
}
