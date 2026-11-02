import type { MusicSearchSelection, SearchResult } from '@/types'
import type { Playlist, Song as ProtocolSong } from '@/types/ws-protocol'
import { ArrowLeft, ArrowRight, CirclePlus, Clock3, Search, Sparkles, X } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useStore } from '@/hooks/use-store'
import { historyStore } from '@/stores/history-store'
import { playerStore } from '@/stores/player-store'
import { searchStore } from '@/stores/search-store'
import { websocketStore } from '@/stores/websocket-store'
import { SongArtwork } from './song-artwork'
import { SongList } from './song-list'
import { Artwork, AsyncIconButton, EmptyState, IconButton, Skeleton } from './ui'

type SearchMode = MusicSearchSelection['mode']
let sessionQuery = ''
let sessionMode: SearchMode = 'all'
let sessionSource: MusicSearchSelection['source'] = 'netease'
try {
  if (JSON.parse(localStorage.getItem('music-modal-platform') || '"netease"') === 'qq')
    sessionSource = 'qq'
} catch {
  // Use the default source for invalid legacy settings.
}

function toSong(song: ProtocolSong): SearchResult {
  return { id: song.id, originalId: song.id, title: song.name, artist: song.artist || '未知艺术家', album: song.album, cover: song.pictureUrl, source: song.source, duration: song.durationMs, webUrl: song.webUrl }
}

function toPlaylist(list: Playlist): SearchResult {
  return { id: list.id, originalId: list.id, title: list.name, artist: list.creator, album: `${list.songCount} 首歌曲`, cover: list.pictureUrl, source: list.source, duration: 0 }
}

export default function SearchPage({ initialQuery, initialSearch, onSearch }: {
  initialQuery?: string
  initialSearch?: MusicSearchSelection
  onSearch: (selection: Omit<MusicSearchSelection, 'id'>) => void
}) {
  const query = initialSearch?.keyword ?? sessionQuery
  const queryLabel = initialSearch?.label || query
  const mode = initialSearch?.mode ?? sessionMode
  const source = initialSearch?.source ?? sessionSource
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [ai, setAi] = useState(false)
  // The field shows the keyword the user typed, also while an opened playlist is on screen.
  const [draft, setDraft] = useState(initialQuery ?? queryLabel)
  const [playlists, setPlaylists] = useState<SearchResult[]>([])
  const seq = useRef(0)
  const consumedInitial = useRef<number>()
  const results = useStore(searchStore, s => s.searchResults)
  const page = useStore(searchStore, s => s.currentPage)
  const totalPages = useStore(searchStore, s => s.totalPages)
  const count = useStore(searchStore, s => s.searchCounts)
  const searchHistory = useStore(historyStore, s => s.searchHistory)
  const historyEnabled = useStore(historyStore, s => s.enableSearchHistory)
  const recommendations = useStore(playerStore, s => s.recommendations)
  const connected = useStore(websocketStore, s => s.connectionStatus === 'connected')

  useEffect(() => () => {
    seq.current++
  }, [])

  const search = useCallback(async (nextPage = 1) => {
    if (!query.trim() || !connected)
      return
    const requestId = ++seq.current
    setBusy(true)
    setError('')
    setAi(false)
    sessionQuery = query
    sessionMode = mode
    sessionSource = source
    localStorage.setItem('music-modal-platform', JSON.stringify(source))
    searchStore.setPage(nextPage)
    historyStore.addToSearchHistory(query, source, mode)
    const data = { keyword: query.trim(), source, pageIndex: nextPage, pageSize: searchStore.state.pageSize }
    const [songsResponse, playlistsResponse] = await Promise.allSettled([
      mode === 'playlist' ? Promise.resolve(null) : websocketStore.request('music.search', data),
      mode === 'song' ? Promise.resolve(null) : websocketStore.request('playlist.search', data),
    ])
    if (requestId !== seq.current)
      return
    const songs = songsResponse.status === 'fulfilled' ? songsResponse.value : null
    const lists = playlistsResponse.status === 'fulfilled' ? playlistsResponse.value : null
    searchStore.updateSearchResults(
      mode === 'playlist' ? (lists?.items.map(toPlaylist) ?? []) : (songs?.items.map(toSong) ?? []),
      mode === 'playlist' ? (lists?.totalSize ?? 0) : (songs?.totalSize ?? 0),
    )
    setPlaylists(lists?.items.map(toPlaylist) ?? [])
    const failures = [songsResponse, playlistsResponse].flatMap(response => response.status === 'rejected'
      ? [response.reason instanceof Error ? response.reason.message : '搜索失败，请重试']
      : [])
    setError([...new Set(failures)].join('；'))
    setBusy(false)
  }, [query, mode, source, connected])

  useEffect(() => {
    if (initialSearch && connected && consumedInitial.current !== initialSearch.id) {
      consumedInitial.current = initialSearch.id
      void search()
    }
  }, [initialSearch, search, connected])

  const refine = (nextMode: SearchMode, nextSource = source) => {
    // Inside an opened playlist the fields keep the typed keyword, so a playlist search looks that
    // keyword up instead of repeating the internal `*id` query, which matches nothing.
    const leavingPlaylist = nextMode === 'playlist' && query.trimStart().startsWith('*')
    onSearch({
      keyword: leavingPlaylist ? (initialQuery ?? queryLabel) : query,
      label: leavingPlaylist ? undefined : initialSearch?.label,
      mode: nextMode,
      source: nextSource,
    })
  }
  const recommend = async () => {
    if (!connected)
      return
    const requestId = ++seq.current
    setAi(true)
    setBusy(true)
    setError('')
    await playerStore.pullRecommendations()
    if (requestId === seq.current)
      setBusy(false)
  }
  const openPlaylist = (list: SearchResult) => {
    onSearch({ keyword: `*${list.id}`, label: list.title, mode: 'song', source: list.source === 'qq' ? 'qq' : 'netease' })
  }
  const featured = results[0]
  const listResults = mode === 'playlist' ? results : playlists
  const hasResults = results.length > 0 || (mode === 'all' && playlists.length > 0)

  return (
    <div className="page-content search-page">
      <div className="search-filter-bar">
        <form
          className="search-inline-field"
          role="search"
          onSubmit={(e) => {
            e.preventDefault()
            const keyword = draft.trim()
            if (connected && keyword)
              onSearch({ keyword, mode: ai ? 'all' : mode, source })
          }}
        >
          <Search size={20} strokeWidth={1.7} aria-hidden="true" />
          <input
            value={draft}
            aria-label="搜索音乐"
            placeholder="歌手、歌曲或歌单"
            disabled={!connected}
            onChange={e => setDraft(e.target.value)}
          />
          {draft && (
            <IconButton
              icon={X}
              label="清空搜索"
              onClick={() => {
                setDraft('')
                onSearch({ keyword: '', mode, source })
              }}
            />
          )}
        </form>
        <div className="search-filter-chips" role="group" aria-label="搜索类别">
          {(['all', 'song', 'playlist'] as const).map(value => (
            <button type="button" key={value} className="search-chip text-ui-caption" aria-pressed={mode === value && !ai} onClick={() => refine(value)}>
              {value === 'all' ? '全部' : value === 'song' ? '歌曲' : '歌单'}
            </button>
          ))}
          <button type="button" className="search-chip text-ui-caption" aria-pressed={ai} disabled={!connected || (busy && ai)} onClick={() => void recommend()}>
            <Sparkles size={14} aria-hidden="true" />
            个性化推荐
          </button>
        </div>
        {!ai && (
          <select className="search-source text-ui-caption" aria-label="音乐平台" value={source} onChange={event => refine(mode, event.target.value === 'qq' ? 'qq' : 'netease')}>
            <option value="netease">网易云音乐</option>
            <option value="qq">QQ音乐</option>
          </select>
        )}
      </div>
      <header className="search-page-heading">
        <h1 className="text-ui-xl">{ai ? '为你推荐' : query ? '搜索结果' : '搜索'}</h1>
        <p className="text-ui-caption muted">
          {ai ? '发现下一首想听的歌。' : query ? `“${queryLabel}” · ${source === 'qq' ? 'QQ音乐' : '网易云音乐'}` : '搜索歌名、歌手或歌单，或从下面的分类开始。'}
        </p>
        {ai && (
          <p className="search-recommend-hint text-ui-caption muted">
            <Sparkles size={14} aria-hidden="true" />
            根据房间播放历史推荐相似歌曲
          </p>
        )}
      </header>
      {error && (
        <div className="search-error" role="alert">
          <p className="error-text text-ui-caption">{error}</p>
          <button type="button" className="button-secondary" disabled={!connected || busy} onClick={() => void search()}>重试</button>
        </div>
      )}
      {!connected && <p className="text-ui-caption muted">加入房间并连接服务器后，即可搜索和点歌。</p>}
      {busy
        ? (
            <div className={`search-loading${mode === 'all' && !ai ? ' search-loading-overview' : ''}`} role="status" aria-label="搜索中">
              {mode === 'all' && !ai && (
                <div className="search-feature-skeleton" aria-hidden="true">
                  <span />
                  <span />
                  <span />
                </div>
              )}
              <Skeleton rows={6} />
            </div>
          )
        : ai
          ? (
              <section aria-label="个性化推荐歌曲">
                <div className="section-heading">
                  <h2 className="text-ui-lg">歌曲</h2>
                  <button type="button" className="search-section-link text-ui-caption" disabled={!connected} onClick={() => void recommend()}>换一批</button>
                </div>
                {recommendations.length
                  ? <SearchSongs songs={recommendations} disabled={!connected} />
                  : <EmptyState icon={Sparkles} title="暂时没有推荐">稍后再获取一组新推荐。</EmptyState>}
              </section>
            )
          : hasResults
            ? (
                <>
                  {mode === 'all'
                    ? (
                        <>
                          {featured && (
                            <div className="search-overview-grid">
                              <section aria-labelledby="search-feature-title">
                                <h2 id="search-feature-title" className="text-ui-lg search-section-title">最佳匹配</h2>
                                <article className="search-featured-song">
                                  <SongArtwork song={featured} />
                                  <h3 className="text-ui-xl truncate" title={featured.title}>{featured.title}</h3>
                                  <p className="text-ui-caption muted truncate" title={featured.artist}>{featured.artist}</p>
                                  <div className="search-feature-footer">
                                    <span className="text-ui-caption">歌曲</span>
                                    <AsyncIconButton icon={CirclePlus} label={`点播首条结果 ${featured.title}`} className="song-action" disabled={!connected} onAction={() => playerStore.pickMusic(featured)} />
                                  </div>
                                </article>
                              </section>
                              <section className="search-preview" aria-labelledby="search-songs-title">
                                <div className="section-heading">
                                  <h2 id="search-songs-title" className="text-ui-lg">歌曲</h2>
                                  <button type="button" className="search-section-link text-ui-caption" onClick={() => refine('song')}>显示全部</button>
                                </div>
                                <SongList songs={results.slice(0, 4)} disabled={!connected} />
                              </section>
                            </div>
                          )}
                          {listResults.length > 0 && (
                            <section className="search-playlists" aria-labelledby="search-playlists-title">
                              <div className="section-heading">
                                <h2 id="search-playlists-title" className="text-ui-lg">歌单</h2>
                                <button type="button" className="search-section-link text-ui-caption" onClick={() => refine('playlist')}>显示全部</button>
                              </div>
                              <PlaylistGrid lists={listResults.slice(0, 5)} disabled={!connected} onOpen={openPlaylist} />
                            </section>
                          )}
                        </>
                      )
                    : (
                        <section aria-label={mode === 'song' ? '歌曲搜索结果' : '歌单搜索结果'}>
                          <div className="section-heading">
                            <h2 className="text-ui-lg">{mode === 'song' ? '歌曲' : '歌单'}</h2>
                            <span className="text-ui-caption muted">
                              {count}
                              {' '}
                              个结果
                            </span>
                          </div>
                          {mode === 'song'
                            ? <SearchSongs songs={results} disabled={!connected} />
                            : <PlaylistGrid lists={results} disabled={!connected} onOpen={openPlaylist} />}
                          <div className="pagination" aria-label="搜索结果分页">
                            <IconButton icon={ArrowLeft} label="上一页" disabled={page <= 1 || !connected} onClick={() => void search(page - 1)} />
                            <span className="text-ui-sm muted">
                              {page}
                              {' '}
                              /
                              {' '}
                              {Math.max(1, totalPages)}
                            </span>
                            <IconButton icon={ArrowRight} label="下一页" disabled={page >= totalPages || !connected} onClick={() => void search(page + 1)} />
                          </div>
                        </section>
                      )}
                </>
              )
            : query
              ? <EmptyState icon={Search} title={error ? '暂时无法搜索' : '没有找到匹配结果'}>试试另一个歌名、歌手或音乐平台。</EmptyState>
              : (
                  <>
                    {historyEnabled && searchHistory.length > 0
                      ? (
                          <section className="search-recent-section" aria-label="最近搜索">
                            <div className="section-heading">
                              <h2 className="text-ui-lg">最近搜索</h2>
                              <button type="button" className="search-section-link text-ui-caption" onClick={() => historyStore.clearSearchHistory()}>清除</button>
                            </div>
                            <div className="search-recent-grid">
                              {searchHistory.slice(0, 8).map(item => (
                                <button type="button" className="search-recent-card" key={`${item.platform}:${item.searchMode}:${item.query}`} disabled={!connected} onClick={() => onSearch({ keyword: item.query, mode: item.searchMode === 'all' ? 'all' : item.searchMode === 'playlist' ? 'playlist' : 'song', source: item.platform === 'qq' ? 'qq' : 'netease' })}>
                                  <Clock3 size={20} aria-hidden="true" />
                                  <span className="song-copy">
                                    <strong className="text-ui-base truncate">{item.query}</strong>
                                    <span className="text-ui-caption muted">{item.platform === 'qq' ? 'QQ音乐' : '网易云音乐'}</span>
                                  </span>
                                  <ArrowRight size={18} aria-hidden="true" />
                                </button>
                              ))}
                            </div>
                          </section>
                        )
                      : <EmptyState icon={Search} title="你想听哪首歌？">在顶部搜索歌名、歌手或歌单。</EmptyState>}
                    <p className="search-help text-ui-caption muted">也可以搜索 *歌单ID 或 @用户ID，查找歌单。</p>
                  </>
                )}
    </div>
  )
}

function SearchSongs({ songs, disabled }: { songs: SearchResult[], disabled: boolean }) {
  return (
    <div className="search-song-table">
      <div className="search-song-heading text-ui-caption muted" aria-hidden="true">
        <span>#</span>
        <span>标题</span>
        <span>专辑</span>
        <span><Clock3 size={16} /></span>
        <span />
      </div>
      <SongList songs={songs} disabled={disabled} />
    </div>
  )
}

function PlaylistGrid({ lists, disabled, onOpen }: { lists: SearchResult[], disabled: boolean, onOpen: (list: SearchResult) => void }) {
  return (
    <div className="album-grid search-album-grid">
      {lists.map(list => (
        <button type="button" className="album-card search-playlist-card" key={`${list.source}:${list.id}`} disabled={disabled} onClick={() => onOpen(list)}>
          <div className="search-playlist-art">
            <Artwork src={list.cover} title={list.title} />
            <span className="search-playlist-open"><ArrowRight size={20} aria-hidden="true" /></span>
          </div>
          <strong className="text-ui-base truncate" title={list.title}>{list.title}</strong>
          <span className="text-ui-caption muted truncate">
            {list.artist || '未知创建者'}
            {' '}
            ·
            {' '}
            {list.album}
          </span>
        </button>
      ))}
    </div>
  )
}
