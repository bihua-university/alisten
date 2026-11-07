import type { MusicSearchSelection, SearchResult } from '@/types'
import type { Playlist, Song as ProtocolSong } from '@/types/livehouse-protocol'
import { cleanup, computed, effect, Scroll, signal, Text, View } from 'nsh'
import { Input } from 'nsh/base'
import { useStore } from '@/hooks/use-store'
import { historyStore } from '@/stores/history-store'
import { playerStore } from '@/stores/player-store'
import { searchStore } from '@/stores/search-store'
import { websocketStore } from '@/stores/websocket-store'
import { brand, card, compact, content, font, foreground, inputBorder, muted, subtle, surface } from '@/theme'
import { searchMusic as searchMusicAPI, searchPlaylists as searchPlaylistsAPI } from '@/utils/api'
import { localStorage } from '@/utils/storage'
import { Icon } from './icon'
import { MusicSourcePicker } from './music-source-picker'
import { SongArtwork } from './song-artwork'
import { SongList } from './song-list'
import { Artwork, AsyncIconButton, EmptyState, IconButton, Skeleton, TextButton } from './ui'

type SearchMode = MusicSearchSelection['mode']
let sessionQuery = ''
let sessionMode: SearchMode = 'all'
let sessionSource: MusicSearchSelection['source'] = 'netease'
try {
  if (JSON.parse(localStorage.getItem('music-modal-platform') || '"netease"') === 'qq')
    sessionSource = 'qq'
} catch {}

function toSong(song: ProtocolSong): SearchResult {
  return { id: song.id, originalId: song.id, title: song.name, artist: song.artist || '未知艺术家', album: song.album, cover: song.pictureUrl, source: song.source, duration: song.durationMs, webUrl: song.webUrl }
}

function toPlaylist(list: Playlist): SearchResult {
  return { id: list.id, originalId: list.id, title: list.name, artist: list.creator, album: `${list.songCount} 首歌曲`, cover: list.pictureUrl, source: list.source, duration: 0 }
}

export default function SearchPage(props: {
  initialQuery?: string
  initialSearch?: MusicSearchSelection
  onSearch: (selection: Omit<MusicSearchSelection, 'id'>) => void
}) {
  const query = () => props.initialSearch?.keyword ?? sessionQuery
  const queryLabel = () => props.initialSearch?.label || query()
  const mode = () => props.initialSearch?.mode ?? sessionMode
  const source = () => props.initialSearch?.source ?? sessionSource
  const busy = signal(false)
  const songsLoading = signal(false)
  const playlistsLoading = signal(false)
  const error = signal('')
  const ai = signal(false)
  const draft = signal(props.initialQuery ?? queryLabel())
  const playlists = signal<SearchResult[]>([])
  const contentWidth = signal(1000)
  let sequence = 0
  let consumedInitial: number | undefined
  const results = useStore(searchStore, state => state.searchResults)
  const page = useStore(searchStore, state => state.currentPage)
  const totalPages = useStore(searchStore, state => state.totalPages)
  const count = useStore(searchStore, state => state.searchCounts)
  const searchHistory = useStore(historyStore, state => state.searchHistory)
  const historyEnabled = useStore(historyStore, state => state.enableSearchHistory)
  const recommendations = useStore(playerStore, state => state.recommendations)
  const connected = useStore(websocketStore, state => state.connectionStatus === 'connected')

  cleanup(() => {
    sequence++
  })

  const search = async (nextPage = 1) => {
    if (!query().trim())
      return
    const requestId = ++sequence
    const searchMode = mode()
    busy.set(true)
    songsLoading.set(searchMode !== 'playlist')
    playlistsLoading.set(searchMode !== 'song')
    searchStore.clearSearchResults()
    playlists.set([])
    error.set('')
    ai.set(false)
    sessionQuery = query()
    sessionMode = mode()
    sessionSource = source()
    localStorage.setItem('music-modal-platform', JSON.stringify(source()))
    searchStore.setPage(nextPage)
    historyStore.addToSearchHistory(query(), source(), mode())
    const data = { keyword: query().trim(), source: source(), pageIndex: nextPage, pageSize: searchStore.state.pageSize }
    // Publish each section as soon as its own request completes. Image loads
    // belong to each Artwork and never participate in this request barrier.
    const [songsResponse, playlistsResponse] = await Promise.allSettled([
      searchMode === 'playlist'
        ? Promise.resolve(null)
        : searchMusicAPI(data).then((songs) => {
            if (requestId === sequence) {
              searchStore.updateSearchResults(songs.items.map(toSong), songs.totalSize)
              songsLoading.set(false)
            }
            return songs
          }),
      searchMode === 'song'
        ? Promise.resolve(null)
        : searchPlaylistsAPI(data).then((lists) => {
            if (requestId === sequence) {
              playlists.set(lists.items.map(toPlaylist))
              if (searchMode === 'playlist')
                searchStore.updateSearchResults(lists.items.map(toPlaylist), lists.totalSize)
              playlistsLoading.set(false)
            }
            return lists
          }),
    ])
    if (requestId !== sequence)
      return
    songsLoading.set(false)
    playlistsLoading.set(false)
    const failures = [songsResponse, playlistsResponse].flatMap(response => response.status === 'rejected'
      ? [response.reason instanceof Error ? response.reason.message : '搜索失败，请重试']
      : [])
    error.set([...new Set(failures)].join('；'))
    busy.set(false)
  }

  effect(() => {
    const initial = props.initialSearch
    if (initial && consumedInitial !== initial.id) {
      consumedInitial = initial.id
      draft.set(props.initialQuery ?? initial.label ?? initial.keyword)
      void search()
    }
  })

  const refine = (nextMode: SearchMode, nextSource = source()) => {
    const leavingPlaylist = nextMode === 'playlist' && query().trimStart().startsWith('*')
    props.onSearch({
      keyword: leavingPlaylist ? (props.initialQuery ?? queryLabel()) : query(),
      label: leavingPlaylist ? undefined : props.initialSearch?.label,
      mode: nextMode,
      source: nextSource,
    })
  }
  const recommend = async () => {
    const requestId = ++sequence
    ai.set(true)
    busy.set(true)
    error.set('')
    await playerStore.pullRecommendations()
    if (requestId === sequence)
      busy.set(false)
  }
  const openPlaylist = (list: SearchResult) => {
    props.onSearch({ keyword: `*${list.id}`, label: list.title, mode: 'song', source: list.source === 'qq' ? 'qq' : 'netease' })
  }
  const featured = () => results()[0]
  const listResults = () => mode() === 'playlist' ? results() : playlists()
  const hasResults = () => results().length > 0 || (mode() === 'all' && playlists().length > 0)
  const mobile = compact
  const columns = computed(() => contentWidth() <= 480 ? 2 : contentWidth() <= 720 ? 3 : 5)

  return (
    <View onLayout={({ bounds }) => contentWidth.set(bounds.size.width)} style={{ width: '100%', maxWidth: 1400, marginHorizontal: 'auto', paddingHorizontal: mobile() ? 16 : 32, paddingTop: mobile() ? 8 : 16, paddingBottom: 48 }}>
      <View style={{ position: 'sticky', top: 0, zIndex: 2, gap: mobile() ? 8 : 16, paddingTop: 8, paddingBottom: mobile() ? 12 : 16, backgroundColor: content }}>
        <If when={mobile()}>
          <View style={{ height: 48, flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 18, borderWidth: 1, borderColor: inputBorder, borderRadius: 24, backgroundColor: surface }}>
            <Icon name="Search" size={20} />
            <Input
              value={draft()}
              accessibilityLabel="搜索音乐"
              placeholder="歌手、歌曲或歌单"
              returnKeyType="search"
              onChange={({ value }) => draft.set(value)}
              onSubmit={() => {
                const keyword = draft().trim()
                if (keyword && connected())
                  props.onSearch({ keyword, mode: ai() ? 'all' : mode(), source: source() })
              }}
              style={{ flex: 1, minWidth: 0, color: foreground, placeholderColor: subtle, fontSize: 16 }}
            />
            <If when={draft()}>
              <IconButton
                icon="X"
                label="清空搜索"
                onPress={() => {
                  draft.set('')
                  props.onSearch({ keyword: '', mode: mode(), source: source() })
                }}
              />
            </If>
          </View>
        </If>
        <View style={{ flexDirection: mobile() ? 'column' : 'row', alignItems: mobile() ? 'stretch' : 'center', justifyContent: 'space-between', gap: 8 }}>
          <Scroll horizontal scrollbar={false} style={{ flex: mobile() ? undefined : 1, minWidth: 0, width: mobile() ? '100%' : undefined, height: mobile() ? 48 : 40 }} contentStyle={{ flexDirection: 'row', alignItems: 'center', gap: 8 }}>
            <FilterChip title="全部" selected={mode() === 'all' && !ai()} onPress={() => refine('all')} />
            <FilterChip title="歌曲" selected={mode() === 'song' && !ai()} onPress={() => refine('song')} />
            <FilterChip title="歌单" selected={mode() === 'playlist' && !ai()} onPress={() => refine('playlist')} />
            <FilterChip title="个性化推荐" icon selected={ai()} disabled={busy() && ai()} onPress={() => void recommend()} />
          </Scroll>
          <If when={!ai()}>
            <MusicSourcePicker
              value={source()}
              onChange={value => refine(mode(), value)}
            />
          </If>
        </View>
      </View>
      <View style={{ marginTop: 16, marginBottom: 28, gap: 6 }}>
        <Text style={[font(14), { color: foreground, fontWeight: '600' }]}>{ai() ? '为你推荐' : query() ? '搜索结果' : '搜索'}</Text>
        <Text style={[font(-1), { color: muted }]}>{ai() ? '发现下一首想听的歌。' : query() ? `“${queryLabel()}” · ${source() === 'qq' ? 'QQ音乐' : '网易云音乐'}` : '搜索歌名、歌手或歌单，或从下面的分类开始。'}</Text>
        <If when={ai() && mobile()}>
          <View style={{ flexDirection: 'row', alignItems: 'center', gap: 6 }}>
            <Icon name="Sparkles" size={14} color={brand} />
            <Text style={[font(-1), { color: muted }]}>根据房间播放历史推荐相似歌曲</Text>
          </View>
        </If>
      </View>
      <If when={error()}>
        <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 16, marginBottom: 24 }}>
          <Text style={[font(-1), { color: '#ff929a', flex: 1 }]}>{error()}</Text>
          <TextButton title="重试" variant="secondary" disabled={busy()} onPress={() => void search()} />
        </View>
      </If>
      <If when={!connected()}>
        <Text style={[font(-1), { color: muted, marginBottom: 16 }]}>搜索与推荐可直接使用；连接房间后即可点歌。</Text>
      </If>
      <If
        when={!busy() || (!ai() && hasResults())}
        else={(
          <View style={{ display: 'grid', gridTemplateColumns: mode() === 'all' && !ai() && contentWidth() > 720 ? 'minmax(240px, .85fr) minmax(0, 1.5fr)' : 'minmax(0, 1fr)', gap: 24 }}>
            <If when={mode() === 'all' && !ai()}>
              <View style={{ gap: 16, padding: 20, minHeight: 260, borderRadius: 12, backgroundColor: card }}>
                <View style={{ width: 88, height: 88, borderRadius: 8, backgroundColor: surface }} />
                <View style={{ width: '60%', height: 14, borderRadius: 4, backgroundColor: surface }} />
                <View style={{ width: '45%', height: 14, borderRadius: 4, backgroundColor: surface }} />
              </View>
            </If>
            <Skeleton rows={6} />
          </View>
        )}
      >
        <If
          when={!ai()}
          else={(
            <View>
              <SectionHeading title="歌曲" action="换一批" onAction={() => void recommend()} />
              <If when={recommendations().length} else={<EmptyState icon="Sparkles" title="暂时没有推荐">稍后再获取一组新推荐。</EmptyState>}>
                <SongList songs={recommendations()} disabled={!connected()} />
              </If>
            </View>
          )}
        >
          <If
            when={hasResults()}
            else={query()
              ? <EmptyState icon="Search" title={error() ? '暂时无法搜索' : '没有找到匹配结果'}>试试另一个歌名、歌手或音乐平台。</EmptyState>
              : (
                  <View>
                    <If when={historyEnabled() && searchHistory().length > 0} else={<EmptyState icon="Search" title="你想听哪首歌？">在顶部搜索歌名、歌手或歌单。</EmptyState>}>
                      <SectionHeading title="最近搜索" action="清除" onAction={() => historyStore.clearSearchHistory()} />
                      <View style={{ display: 'grid', gridTemplateColumns: contentWidth() <= 480 ? 'minmax(0, 1fr)' : 'repeat(2, minmax(0, 1fr))', gap: 12 }}>
                        <Each of={searchHistory().slice(0, 8)} key={item => `${item.platform}:${item.searchMode}:${item.query}`}>
                          {item => (
                            <View role="button" class="cursor-pointer" accessibilityLabel={item().query} onPress={() => props.onSearch({ keyword: item().query, mode: item().searchMode === 'all' ? 'all' : item().searchMode === 'playlist' ? 'playlist' : 'song', source: item().platform === 'qq' ? 'qq' : 'netease' })} style={{ flexDirection: 'row', alignItems: 'center', gap: 16, padding: 16, borderRadius: 8, backgroundColor: card }}>
                              <Icon name="Clock3" size={20} />
                              <View style={{ flex: 1, minWidth: 0, gap: 3 }}>
                                <Text style={[font(), { color: foreground, fontWeight: '600', numberOfLines: 1 }]}>{item().query}</Text>
                                <Text style={[font(-1), { color: muted }]}>{item().platform === 'qq' ? 'QQ音乐' : '网易云音乐'}</Text>
                              </View>
                              <Icon name="ArrowRight" size={18} />
                            </View>
                          )}
                        </Each>
                      </View>
                    </If>
                    <Text style={[font(-1), { color: muted, marginTop: 24 }]}>也可以搜索 *歌单ID 或 @用户ID，查找歌单。</Text>
                  </View>
                )}
          >
            <If
              when={mode() === 'all'}
              else={(
                <View>
                  <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
                    <Text style={[font(4), { color: foreground, fontWeight: '600' }]}>{mode() === 'song' ? '歌曲' : '歌单'}</Text>
                    <Text style={[font(-1), { color: muted }]}>{`${count()} 个结果`}</Text>
                  </View>
                  <If when={mode() === 'song'} else={<PlaylistGrid lists={results()} columns={columns()} onOpen={openPlaylist} />}>
                    <SongList songs={results()} disabled={!connected()} />
                  </If>
                  <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 20, paddingTop: 24 }}>
                    <IconButton icon="ArrowLeft" label="上一页" disabled={page() <= 1 || !connected()} onPress={() => void search(page() - 1)} />
                    <Text style={[font(-2), { color: muted }]}>{`${page()} / ${Math.max(1, totalPages())}`}</Text>
                    <IconButton icon="ArrowRight" label="下一页" disabled={page() >= totalPages() || !connected()} onPress={() => void search(page() + 1)} />
                  </View>
                </View>
              )}
            >
              <If when={songsLoading()}>
                <View style={{ marginBottom: 24 }}>
                  <SectionHeading title="歌曲" />
                  <Skeleton rows={4} />
                </View>
              </If>
              <If when={featured()}>
                {match => (
                  <View style={{ display: 'grid', gridTemplateColumns: contentWidth() <= 720 ? 'minmax(0, 1fr)' : 'minmax(240px, .85fr) minmax(0, 1.5fr)', gap: 24 }}>
                    <View>
                      <Text style={[font(4), { color: foreground, fontWeight: '600', marginBottom: 16 }]}>最佳匹配</Text>
                      <View style={{ padding: 20, borderRadius: 12, backgroundColor: card }}>
                        <SongArtwork song={match()} size={88} />
                        <Text style={[font(14), { color: foreground, fontWeight: '600', marginTop: 16, numberOfLines: 1 }]}>{match().title}</Text>
                        <Text style={[font(-1), { color: muted, numberOfLines: 1 }]}>{match().artist}</Text>
                        <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginTop: 6 }}>
                          <Text style={[font(-1), { color: foreground }]}>歌曲</Text>
                          <AsyncIconButton icon="CirclePlus" label={`点播首条结果 ${match().title}`} color={brand} disabled={!connected()} onAction={() => playerStore.pickMusic(match())} />
                        </View>
                      </View>
                    </View>
                    <View>
                      <SectionHeading title="歌曲" action="显示全部" onAction={() => refine('song')} />
                      <SongList songs={results().slice(0, 4)} disabled={!connected()} compact />
                    </View>
                  </View>
                )}
              </If>
              <If when={listResults().length > 0}>
                <View style={{ marginTop: 32 }}>
                  <SectionHeading title="歌单" action="显示全部" onAction={() => refine('playlist')} />
                  <PlaylistGrid lists={listResults().slice(0, mobile() ? 4 : 5)} columns={mobile() ? 2 : columns()} onOpen={openPlaylist} />
                </View>
              </If>
              <If when={playlistsLoading()}>
                <View style={{ marginTop: 32 }}>
                  <SectionHeading title="歌单" />
                  <Skeleton rows={2} />
                </View>
              </If>
            </If>
          </If>
        </If>
      </If>
    </View>
  )
}

function FilterChip(props: { title: string, selected: boolean, disabled?: boolean, icon?: boolean, onPress: () => void }) {
  return (
    <View
      role="button"
      class="cursor-pointer"
      accessibilityLabel={props.title}
      accessibilityState={{ selected: props.selected, disabled: props.disabled }}
      onPress={props.disabled ? undefined : props.onPress}
      style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 6, minHeight: compact() ? 44 : 34, paddingHorizontal: 14, paddingVertical: 7, borderRadius: 999, backgroundColor: props.selected ? foreground : surface, opacity: props.disabled ? 0.45 : 1, flexShrink: 0 }}
    >
      <If when={props.icon}>
        <Icon name="Sparkles" size={14} color={props.selected ? content : brand} />
      </If>
      <Text style={[font(-1), { color: props.selected ? content : foreground }]}>{props.title}</Text>
    </View>
  )
}

function SectionHeading(props: { title: string, action?: string, onAction?: () => void }) {
  return (
    <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12, marginBottom: 16 }}>
      <Text style={[font(4), { color: foreground, fontWeight: '600' }]}>{props.title}</Text>
      <If when={props.action}>
        <View role="button" class="cursor-pointer" accessibilityLabel={props.action} onPress={props.onAction} style={{ paddingVertical: 4 }}>
          <Text style={[font(-1), { color: muted }]}>{props.action}</Text>
        </View>
      </If>
    </View>
  )
}

function PlaylistGrid(props: { lists: SearchResult[], columns: number, onOpen: (list: SearchResult) => void }) {
  return (
    <View style={{ display: 'grid', gridTemplateColumns: `repeat(${props.columns}, minmax(0, 1fr))`, gap: 12 }}>
      <Each of={props.lists} key={list => `${list.source}:${list.id}`}>
        {list => (
          <View role="button" accessibilityLabel={list().title} class="group cursor-pointer transition-colors duration-150" onPress={() => props.onOpen(list())} style={{ minWidth: 0, gap: 5, padding: 10, borderRadius: 12 }}>
            <View style={{ position: 'relative', width: '100%' }}>
              <PlaylistArtwork src={list().cover} title={list().title} />
              <View class="opacity-0 group-hover:opacity-100 group-focus:opacity-100" style={{ position: 'absolute', right: 10, bottom: 16, width: 36, height: 36, borderRadius: 999, alignItems: 'center', justifyContent: 'center', backgroundColor: brand }}>
                <Icon name="ArrowRight" size={20} color="#122412" />
              </View>
            </View>
            <Text style={[font(), { color: foreground, fontWeight: '600', numberOfLines: 1 }]}>{list().title}</Text>
            <Text style={[font(-1), { color: muted, numberOfLines: 1 }]}>{`${list().artist || '未知创建者'} · ${list().album}`}</Text>
          </View>
        )}
      </Each>
    </View>
  )
}

function PlaylistArtwork(props: { src: string, title: string }) {
  return (
    <View style={{ width: '100%', aspectRatio: 1 }}>
      <Artwork src={props.src} title={props.title} size={88} fill />
    </View>
  )
}
