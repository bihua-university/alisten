package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/bihua-university/alisten/cmd/room-service/internal/config"
	"github.com/bihua-university/alisten/cmd/room-service/internal/rpc/musichub"
	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/nacos"
	"github.com/bihua-university/alisten/protocol"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var (
	musicClient  *musichub.Client
	debugEnabled bool
)

// Only override dependency discovery; room registration and config remain isolated.
func discoverMusichubOnce(ctx context.Context) (string, error) {
	if namespace := strings.TrimSpace(os.Getenv("MUSICHUB_NACOS_NAMESPACE")); namespace != "" {
		return nacos.DiscoverInNamespace(ctx, "musichub", namespace)
	}
	return nacos.Discover(ctx, "musichub")
}

func discoverMusichub(ctx context.Context) (string, error) {
	for {
		address, err := discoverMusichubOnce(ctx)
		if err == nil {
			return address, nil
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
}

func main() {
	if err := nacos.Init("room-service"); err != nil {
		logx.Fatal("initialize Nacos:", err)
	}
	namespace := strings.TrimSpace(os.Getenv("MUSICHUB_NACOS_NAMESPACE"))
	if namespace == "" {
		namespace = "(room-service namespace)"
	}
	logx.Info("musichub discovery namespace: ", namespace)
	debugEnabled = debugMode()
	logx.SetDebug(debugEnabled)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.ServerConfig.Get()
	discoveryCtx, cancelDiscovery := context.WithTimeout(ctx, 30*time.Second)
	address, err := discoverMusichub(discoveryCtx)
	cancelDiscovery()
	if err != nil {
		logx.Fatal("discover musichub:", err)
	}
	musicClient = musichub.NewDiscoveryClient(address, discoverMusichubOnce, 0)
	defer musicClient.Close()
	for _, house := range cfg.Persist {
		createHouse(house.ID, house.Name, house.Desc, house.Password, true)
	}

	listener, err := net.Listen("tcp", ":"+port("8081"))
	if err != nil {
		logx.Fatal("listen room-service:", err)
	}
	if err := nacos.Register(ctx, "", listener.Addr().String()); err != nil {
		logx.Fatal("register room-service:", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := nacos.Deregister(ctx, "", listener.Addr().String()); err != nil {
			logx.Error("deregister room-service:", err)
		}
	}()

	server := &http.Server{Handler: newHandler(), ReadHeaderTimeout: 10 * time.Second}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	logx.Info("room-service HTTP ", listener.Addr())
	select {
	case <-ctx.Done():
	case err := <-serveErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logx.Error("room-service serve:", err)
		}
		stop()
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logx.Error("room-service shutdown:", err)
	}
}

func port(fallback string) string {
	if value := strings.TrimSpace(os.Getenv("PORT")); value != "" {
		return value
	}
	return fallback
}

func debugMode() bool {
	value := strings.TrimSpace(os.Getenv("DEBUG"))
	if value == "" {
		return false
	}
	debug, err := strconv.ParseBool(value)
	if err != nil {
		logx.Fatal("parse DEBUG:", err)
	}
	return debug
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
	remoteAddr := r.RemoteAddr
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		remoteAddr = forwarded
	}
	ip := maskIP(remoteAddr)
	conn := &Connection{
		conn: wc, ip: ip, user: auth.User{Name: "游客(" + ip + ")"},
		send: make(chan []byte, connectionSendBuffer), done: make(chan struct{}),
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
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "房间不存在"})
			return
		}
		if house.Password != msg.Get("password").String() {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "密码错误"})
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
