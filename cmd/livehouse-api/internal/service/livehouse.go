// Package service implements the livehouse-api HTTP handlers.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"hash/maphash"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/music"
	pb "github.com/bihua-university/alisten/internal/proto/musichub"
	livehouseRPC "github.com/bihua-university/alisten/internal/rpc/livehouse"
)

type MusicBackend interface {
	Load(context.Context, *pb.SongRequest) (*pb.Song, error)
	Search(context.Context, *pb.SearchRequest) (pb.MusicResult, error)
	Playlists(context.Context, *pb.SearchRequest) (pb.PlaylistResult, error)
	SongList(context.Context, *pb.SearchRequest) (pb.MusicResult, error)
	Similar(context.Context, string) ([]string, error)
}

type handler struct {
	music        MusicBackend
	livehouse    *livehouseRPC.Client
	debug        bool
	resolveGroup flightGroup[uint64, PlayableSong]
	resolveCache *ttlCache[uint64, PlayableSong]
	resolveSeed  maphash.Seed
}

func NewHandler(musicBackend MusicBackend, livehouse *livehouseRPC.Client, debug bool) http.Handler {
	h := &handler{
		music: musicBackend, livehouse: livehouse, debug: debug,
		resolveCache: newTTLCache[uint64, PlayableSong](5 * time.Second),
		resolveSeed:  maphash.MakeSeed(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/house/add", h.addRoom)
	mux.HandleFunc("/api/house/search", h.searchRooms)
	mux.HandleFunc("POST /api/music/pick", h.pickMusic)
	mux.HandleFunc("POST /api/music/search", h.searchMusic)
	mux.HandleFunc("POST /api/music/searchsonglist", h.searchPlaylists)
	mux.HandleFunc("POST /api/music/recommend", h.recommendMusic)
	mux.HandleFunc("POST /api/music/resolve", h.resolveMusic)
	return corsMiddleware(logMiddleware(mux))
}

func (h *handler) addRoom(w http.ResponseWriter, r *http.Request) {
	var request CreateRoomRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	var room RoomInfo
	if !h.writeRPC(w, r, "room.create", RoomRequest{}, request, &room) {
		return
	}
	writeJSON(w, http.StatusOK, CreateRoomResponse{HouseID: room.ID})
}

func (h *handler) searchRooms(w http.ResponseWriter, r *http.Request) {
	var rooms []RoomInfo
	if !h.writeRPC(w, r, "room.list", RoomRequest{}, struct{}{}, &rooms) {
		return
	}
	keyword := strings.ToLower(r.URL.Query().Get("keyword"))
	filtered := make([]RoomInfo, 0, len(rooms))
	for _, room := range rooms {
		if keyword == "" || strings.Contains(strings.ToLower(room.Name), keyword) || strings.Contains(strings.ToLower(room.Description), keyword) {
			filtered = append(filtered, room)
		}
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (h *handler) writeRPC(w http.ResponseWriter, r *http.Request, operation string, room RoomRequest, input any, target any) bool {
	if err := h.livehouse.Call(r.Context(), operation, room.HouseID, room.Password, input, target); err != nil {
		writeRPCError(w, err)
		return false
	}
	return true
}

func writeRPCError(w http.ResponseWriter, err error) {
	var rpcErr *livehouseRPC.Error
	if errors.As(err, &rpcErr) {
		writeJSON(w, rpcErr.Status, ErrorResponse{Error: rpcErr.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "livehouse 请求失败"})
}

func (h *handler) pickMusic(w http.ResponseWriter, r *http.Request) {
	var request PickRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	source := music.ParseSource(request.Source)
	if source == music.Unknown || source.String() != request.Source {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "不支持的音乐来源"})
		return
	}
	id := strings.TrimSpace(request.ID)
	if id == "" {
		if strings.TrimSpace(request.Name) == "" {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "歌曲 ID 和名称至少填写一项"})
			return
		}
		result, err := h.performMusicSearch(r.Context(), SearchRequest{Source: request.Source, Keyword: request.Name, Page: 1, PageSize: 10})
		if err != nil || len(result.Data) == 0 || result.Data[0] == nil || result.Data[0].Id == "" {
			writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "点歌失败，无法搜索到音乐"})
			return
		}
		id = result.Data[0].Id
	}
	song, err := h.music.Load(r.Context(), &pb.SongRequest{Source: request.Source, Id: id, UseCache: true})
	if err != nil || song == nil || song.Url == "" {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "点歌失败，无法获取音乐信息"})
		return
	}
	user := ProtocolUser{Name: "HTTP API"}
	if request.User != nil {
		user.Name = strings.TrimSpace(request.User.Name)
		if user.Name == "" {
			user.Name = "HTTP API"
		}
		if request.User.Email != "" {
			user.EmailHash = auth.EmailToMD5(request.User.Email)
		}
	}
	track := Track{
		Playable: PlayableSong{
			ID: id, Source: request.Source,
			Name:       song.Name,
			Artist:     song.Artist,
			Album:      song.Album,
			DurationMS: max(song.Duration, 0),
			PictureURL: song.PictureUrl,
			WebURL:     song.WebUrl,
			URL:        "",
			Lyric:      song.Lyric,
		},
		User: user, Likes: 0, Voters: []string{},
	}
	room := RoomRequest{HouseID: request.HouseID, Password: request.Password}
	var result PickResult
	if !h.writeRPC(w, r, "queue.enqueue", room, track, &result) {
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) performMusicSearch(ctx context.Context, request SearchRequest) (pb.MusicResult, error) {
	option := &pb.SearchRequest{
		Id:       request.ID,
		Source:   int32(music.ParseSource(request.Source)),
		Keyword:  request.Keyword,
		Page:     request.Page,
		PageSize: request.PageSize,
	}
	if strings.HasPrefix(request.Keyword, "*") {
		option.Id = strings.TrimPrefix(request.Keyword, "*")
		return h.music.SongList(ctx, option)
	}
	return h.music.Search(ctx, option)
}

func (h *handler) searchMusic(w http.ResponseWriter, r *http.Request) {
	var request SearchRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	result, err := h.performMusicSearch(r.Context(), request)
	if err != nil {
		h.writeProviderError(w, err)
		return
	}
	items := make([]ProtocolSong, 0, len(result.Data))
	for _, song := range result.Data {
		if song != nil {
			items = append(items, protocolSongFromMusic(song))
		}
	}
	writeJSON(w, http.StatusOK, ItemsResponse[ProtocolSong]{Items: items, TotalSize: result.Total})
}

func protocolSongFromMusic(song *pb.Music) ProtocolSong {
	return ProtocolSong{
		ID:         song.Id,
		Source:     sourceName(song.Source),
		Name:       song.Name,
		Artist:     song.Artist,
		Album:      song.Album,
		DurationMS: max(song.Duration, 0),
		PictureURL: song.Cover,
	}
}

func (h *handler) searchPlaylists(w http.ResponseWriter, r *http.Request) {
	var request SearchRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	result, err := h.music.Playlists(r.Context(), &pb.SearchRequest{
		Source:   int32(music.ParseSource(request.Source)),
		Keyword:  request.Keyword,
		Page:     request.Page,
		PageSize: request.PageSize,
	})
	if err != nil {
		h.writeProviderError(w, err)
		return
	}
	items := make([]Playlist, 0, len(result.Data))
	for _, playlist := range result.Data {
		if playlist != nil {
			items = append(items, Playlist{
				ID:          playlist.Id,
				Source:      request.Source,
				Name:        playlist.Name,
				PictureURL:  playlist.PictureUrl,
				Description: playlist.Desc,
				Creator:     playlist.Creator,
				CreatorID:   playlist.CreatorUid,
				PlayCount:   playlist.PlayCount,
				SongCount:   playlist.SongCount,
			})
		}
	}
	writeJSON(w, http.StatusOK, ItemsResponse[Playlist]{Items: items, TotalSize: result.Total})
}

func (h *handler) recommendMusic(w http.ResponseWriter, r *http.Request) {
	var request RecommendRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	id := request.ID
	if id == "" {
		room := RoomRequest{HouseID: request.HouseID, Password: request.Password}
		var queue QueueData
		if !h.writeRPC(w, r, "queue.get", room, struct{}{}, &queue) {
			return
		}
		if queue.Current != nil && queue.Current.Song.Source == "netease" {
			id = queue.Current.Song.ID
		} else {
			for _, item := range queue.Items {
				if item.Song.Source == "netease" {
					id = item.Song.ID
					break
				}
			}
		}
	}
	items := make([]ProtocolSong, 0)
	if id != "" {
		ids, err := h.music.Similar(r.Context(), id)
		if err != nil {
			h.writeProviderError(w, err)
			return
		}
		for _, recommendedID := range ids {
			song, err := h.music.Load(r.Context(), &pb.SongRequest{Source: "netease", Id: recommendedID, UseCache: true})
			if err == nil && song != nil {
				items = append(items, ProtocolSong{
					ID:         recommendedID,
					Source:     "netease",
					Name:       song.Name,
					Artist:     song.Artist,
					Album:      song.Album,
					DurationMS: max(song.Duration, 0),
					PictureURL: song.PictureUrl,
					WebURL:     song.WebUrl,
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, ItemsResponse[ProtocolSong]{Items: items, TotalSize: int64(len(items))})
}

func (h *handler) resolveKey(source, id string) uint64 {
	var hash maphash.Hash
	hash.SetSeed(h.resolveSeed)
	_, _ = hash.WriteString(source)
	_ = hash.WriteByte(0)
	_, _ = hash.WriteString(id)
	return hash.Sum64()
}

func (h *handler) resolveMusic(w http.ResponseWriter, r *http.Request) {
	var request ResolveRequest
	if !decodeRequest(w, r, &request) {
		return
	}
	source := music.ParseSource(request.Source)
	if source == music.Unknown || source.String() != request.Source || strings.TrimSpace(request.ID) == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "歌曲来源或 ID 无效"})
		return
	}
	key := h.resolveKey(request.Source, request.ID)
	if cached, ok := h.resolveCache.get(key); ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}
	result, err := h.resolveGroup.do(r.Context(), key, func(ctx context.Context) (PlayableSong, error) {
		if cached, ok := h.resolveCache.get(key); ok {
			return cached, nil
		}
		song, err := h.music.Load(ctx, &pb.SongRequest{Source: request.Source, Id: request.ID, UseCache: false})
		if err != nil || song == nil || song.Url == "" {
			return PlayableSong{}, errors.New("无法获取最新播放地址")
		}
		resolved := PlayableSong{
			ID:         request.ID,
			Source:     request.Source,
			Name:       song.Name,
			Artist:     song.Artist,
			Album:      song.Album,
			DurationMS: max(song.Duration, 0),
			PictureURL: song.PictureUrl,
			WebURL:     song.WebUrl,
			URL:        song.Url,
			Lyric:      song.Lyric,
		}
		if song.Gain.IsSome() {
			gain := song.Gain.Unwrap()
			resolved.Gain = &gain
		}
		h.resolveCache.set(key, resolved)
		return resolved, nil
	})
	if err != nil {
		h.writeProviderError(w, err)
		return
	}
	if result.Source != request.Source || result.ID != request.ID {
		h.writeProviderError(w, errors.New("歌曲解析键冲突"))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func sourceName(source int32) string {
	switch music.Source(source) {
	case music.QQ:
		return "qq"
	case music.NetEase:
		return "netease"
	case music.KuWo:
		return "kuwo"
	default:
		return "netease"
	}
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1024*1024))
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "请求数据格式错误"})
		return false
	}
	return true
}

func (h *handler) writeProviderError(w http.ResponseWriter, err error) {
	message := "音乐服务暂不可用"
	if h.debug && err != nil {
		message = err.Error()
	}
	writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "token,content-type,accesstoken,authorization")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Accept-Ranges, Content-Range")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r)
		logx.Infof("%s \"%s %s\" %d", r.RemoteAddr, r.Method, r.URL.Path, writer.status)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
