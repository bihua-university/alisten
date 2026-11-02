import type { Song } from '@/types'
import type { Source } from '@/types/ws-protocol'
import { CirclePlus, Heart, Trash2 } from 'lucide-react'
import { memo } from 'react'
import { playerStore } from '@/stores/player-store'
import { websocketStore } from '@/stores/websocket-store'
import { formatTime } from '@/utils/time'
import { SongArtwork } from './song-artwork'
import { AsyncIconButton } from './ui'

export const SongRow = memo(
  ({
    song,
    index,
    queue = false,
    disabled = false,
  }: {
    song: Song
    index: number
    queue?: boolean
    disabled?: boolean
  }) => {
    return (
      <li className="song-row">
        <span className="song-index text-ui-sm muted">
          {String(index + 1).padStart(2, '0')}
        </span>
        <SongArtwork song={song} />
        <div className="song-copy">
          <strong className="text-ui-base truncate">{song.title}</strong>
          <span className="text-ui-sm muted truncate">
            {song.artist}
            {song.requestedBy ? ` · ${song.requestedBy.name} 点播` : ''}
          </span>
        </div>
        <span className="song-album text-ui-sm muted truncate">
          {song.album}
        </span>
        <span className="song-duration text-ui-sm muted">
          {formatTime(song.duration / 1000)}
        </span>
        {queue
          ? (
              <>
                <AsyncIconButton
                  icon={Heart}
                  className="song-action"
                  label={`赞 ${song.title}`}
                  disabled={disabled || !song.id}
                  onAction={() =>
                    websocketStore.sendSongLike(
                      song.id!,
                      (song.source || 'netease') as Source,
                    )}
                />
                <AsyncIconButton
                  icon={Trash2}
                  className="song-action"
                  label={`移除 ${song.title}`}
                  disabled={disabled || !song.id}
                  onAction={() =>
                    websocketStore.sendDeleteSong(
                      song.id!,
                      (song.source || 'netease') as Source,
                    )}
                />
              </>
            )
          : (
              <AsyncIconButton
                icon={CirclePlus}
                className="song-action"
                label={`点播 ${song.title}`}
                disabled={disabled}
                onAction={() => playerStore.pickMusic(song)}
              />
            )}
      </li>
    )
  },
)

export function SongList({
  songs,
  queue = false,
  disabled = false,
}: {
  songs: Song[]
  queue?: boolean
  disabled?: boolean
}) {
  return (
    <ol className="song-list">
      {songs.map((song, index) => (
        <SongRow
          key={[song.source, song.id || song.title, song.artist].join(':')}
          song={song}
          index={index}
          queue={queue}
          disabled={disabled}
        />
      ))}
    </ol>
  )
}
