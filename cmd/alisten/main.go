package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/mpipe"
	"github.com/bihua-university/alisten/internal/music"
	"github.com/bihua-university/alisten/internal/syncx"
	"github.com/bihua-university/alisten/internal/task"
	"github.com/bihua-university/alisten/protocol"

	"github.com/caddyserver/certmagic"
	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	base.InitConfig()
	logx.SetDebug(base.Config.Debug)
	music.Init()
	task.Scheduler = task.NewServer(base.Config.Token)
	handler := newHandler()
	for _, house := range base.Config.Persist {
		createHouse(house.ID, house.Name, house.Desc, house.Password, true)
	}
	if base.Config.Debug {
		logx.Fatal(http.ListenAndServe(":8080", handler))
	} else {
		certmagic.HTTPS([]string{base.Config.Addr}, handler)
	}
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/house/add", addHouseHTTP)
	mux.HandleFunc("/house/enter", enterHouseHTTP)
	mux.HandleFunc("/house/search", searchHousesHTTP)
	mux.HandleFunc("POST /house/houseuser", wrapWebsocket(houseuser))
	mux.HandleFunc("POST /music/playlist", wrapWebsocket(getPlaylist))
	mux.HandleFunc("POST /music/sync", wrapWebsocket(getCurrentMusic))
	mux.HandleFunc("POST /music/pick", wrapWebsocket(pickMusic))
	mux.HandleFunc("POST /music/delete", wrapWebsocket(deleteMusic))
	mux.HandleFunc("POST /music/good", wrapWebsocket(goodMusic))
	mux.HandleFunc("POST /music/skip/vote", wrapWebsocket(voteSkip))
	mux.HandleFunc("POST /music/search", wrapWebsocket(searchMusic))
	mux.HandleFunc("POST /music/searchsonglist", wrapWebsocket(searchList))
	mux.HandleFunc("POST /music/playmode", wrapWebsocket(playMode))

	musicPipe := mpipe.New(base.Config.Cookie, 0, 0)
	mux.HandleFunc("GET /music/file", musicPipe.ServeHTTP)
	mux.HandleFunc("GET /tasks/poll", task.Scheduler.PollTaskHandler)
	mux.HandleFunc("POST /tasks/result", task.Scheduler.SubmitResultHandler)
	mux.HandleFunc("GET /config/cookie", getCookieHTTP)
	mux.HandleFunc("POST /config/cookie", func(w http.ResponseWriter, r *http.Request) {
		setCookieHTTP(w, r, musicPipe)
	})
	mux.HandleFunc("/server", serveWebsocket)
	return corsMiddleware(logMiddleware(mux))
}

func serveWebsocket(w http.ResponseWriter, r *http.Request) {
	house := GetHouse(r.URL.Query().Get("houseId"))
	if house == nil || house.Password != r.URL.Query().Get("housePwd") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	wc, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logx.Error("upgrade:", err)
		return
	}
	wc.SetReadLimit(64 * 1024)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	ip := maskIP(r.RemoteAddr)
	conn := &Connection{
		conn: wc, ip: ip, user: auth.User{Name: "游客(" + ip + ")"},
		send: syncx.NewUnboundedChan[[]byte](8), done: make(chan struct{}),
	}
	conn.Start()
	defer conn.Close()
	if !house.enter(conn) {
		return
	}
	defer house.Leave(conn)

	// Bound asynchronous requests while preserving correlation and event delivery.
	pending := make(chan struct{}, 16)
	for {
		frameType, raw, err := wc.ReadMessage()
		if err != nil {
			return
		}
		if frameType != websocket.TextMessage || !utf8.Valid(raw) {
			conn.Send(protocol.Event(protocol.EventProtocolError, protocol.Error{
				Code: protocol.ErrorCodeInvalidMessage, Message: "仅接受 UTF-8 JSON 文本消息",
			}))
			continue
		}
		message, protocolErr := protocol.DecodeRequest(raw)
		if protocolErr != nil {
			conn.Send(protocol.Failure(message, *protocolErr))
			continue
		}
		select {
		case pending <- struct{}{}:
			go func(message protocol.Incoming) {
				defer func() { <-pending }()
				handleRequest(ctx, conn, house, message)
			}(message)
		default:
			conn.Send(protocol.Failure(message, protocol.Error{
				Code: protocol.ErrorCodeRateLimited, Message: "未完成请求过多，请稍后再试",
			}))
		}
	}
}

func handleRequest(ctx context.Context, conn *Connection, house *House, message protocol.Incoming) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	c := &Context{ctx: ctx, conn: conn, house: house, message: message, data: gjson.ParseBytes(message.Data)}
	defer func() {
		if err := recover(); err != nil {
			logx.Error(err, "\n", string(debug.Stack()))
			c.Fail(protocol.ErrorCodeInternalError, "处理请求时发生内部错误")
		}
		if !c.replied {
			c.Fail(protocol.ErrorCodeInternalError, "请求处理未返回结果")
		}
	}()
	handler := route[message.Type]
	if handler == nil {
		c.Fail(protocol.ErrorCodeInternalError, "协议请求尚未接入处理器")
		return
	}
	handler(c)
}

var route = map[string]func(*Context){
	protocol.RequestHeartbeat:        heartbeat,
	protocol.RequestChatSend:         chat,
	protocol.RequestUserUpdate:       setUser,
	protocol.RequestSettingsGet:      settingSync,
	protocol.RequestSettingsSet:      playMode,
	protocol.RequestMembersGet:       houseuser,
	protocol.RequestMusicSearch:      searchMusic,
	protocol.RequestPlaylistSearch:   searchList,
	protocol.RequestMusicPick:        pickMusic,
	protocol.RequestQueueRemove:      deleteMusic,
	protocol.RequestQueueLike:        goodMusic,
	protocol.RequestPlaybackSkipVote: voteSkip,
	protocol.RequestPlaybackGet:      getCurrentMusic,
	protocol.RequestQueueGet:         getPlaylist,
	protocol.RequestMusicRecommend:   recommendMusic,
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "token,content-type,accesstoken,authorization")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Accept-Ranges, Content-Range")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type logResponseWriter struct {
	w http.ResponseWriter
	r *http.Request
}

func (l *logResponseWriter) Header() http.Header {
	return l.w.Header()
}

func (l *logResponseWriter) Write(b []byte) (int, error) {
	return l.w.Write(b)
}

func (l *logResponseWriter) WriteHeader(statusCode int) {
	logx.Infof("%s \"%s %s\" %d", l.r.RemoteAddr, l.r.Method, l.r.URL.Path, statusCode)
	l.w.WriteHeader(statusCode)
}

func (l *logResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := l.w.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer is not a hijacker")
	}
	return hj.Hijack()
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &logResponseWriter{w: w, r: r}
		next.ServeHTTP(lrw, r)
	})
}

func maskIP(ip string) string {
	// 192.0.0.1:80 => 192.0.*.*
	ip = lastCut(ip, ":")
	ip = lastCut(ip, ".")
	ip = lastCut(ip, ".")
	return ip + ".*.*"
}

func lastCut(s, sep string) string {
	if i := strings.LastIndex(s, sep); i >= 0 {
		return s[:i]
	}
	return s
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func wrapWebsocket(fn func(*Context)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		msg := gjson.ParseBytes(body)

		house := GetHouse(msg.Get("houseId").String())
		if house == nil {
			writeJSON(w, http.StatusNotFound, base.H{"error": "房间不存在"})
			return
		}
		if house.Password != msg.Get("password").String() {
			writeJSON(w, http.StatusUnauthorized, base.H{"error": "密码错误"})
			return
		}

		ctx := &Context{
			ctx:   r.Context(),
			hw:    w,
			house: house,
			data:  msg,
		}
		fn(ctx)
	}
}

func localhost() string {
	if base.Config.Debug {
		return "http://localhost:8080"
	} else {
		return "https://" + base.Config.Addr
	}
}
