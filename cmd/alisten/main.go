package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/snapshot"
	"github.com/bihua-university/alisten/internal/syncx"
	"github.com/bihua-university/alisten/internal/task"

	"github.com/caddyserver/certmagic"
	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
} // use default options

func main() {
	base.InitConfig()

	task.Scheduler = task.NewServer(base.Config.Token) // 可以从配置文件读取token

	// 创建HTTP multiplexer
	mux := http.NewServeMux()

	// 添加CORS中间件
	handler := logMiddleware(mux)
	handler = corsMiddleware(handler)

	// 房间相关路由
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

	// task long-polling
	mux.HandleFunc("GET /tasks/poll", task.Scheduler.PollTaskHandler)
	mux.HandleFunc("POST /tasks/result", task.Scheduler.SubmitResultHandler)

	mux.HandleFunc("/server", func(w http.ResponseWriter, r *http.Request) {
		houseId := r.URL.Query().Get("houseId")
		password := r.URL.Query().Get("housePwd")

		house := GetHouse(houseId)
		if house == nil || house.Password != password {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		wc, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		defer wc.Close()

		ip := maskIP(r.RemoteAddr)
		conn := &Connection{
			conn: wc,
			ip:   ip,
			user: auth.User{
				Name: "游客(" + ip + ")",
			},
			send: syncx.NewUnboundedChan[[]byte](8),
		}

		house.Mu.Lock()
		house.Connection = append(house.Connection, conn)
		house.lastActiveTime = time.Now()
		house.Mu.Unlock()

		conn.Start()
		house.enter(conn)

		for {
			_, message, err := wc.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				// remove from connections and broadcast updated user list
				house.Leave(conn)
				break
			}

			// async handle command
			go func() {
				defer func() {
					// prevent crash
					if err := recover(); err != nil {
						log.Println(err, "\n", string(debug.Stack()))
					}
				}()

				msg := gjson.ParseBytes(message)
				handler := route[msg.Get("action").String()]

				if base.Config.Debug {
					fmt.Println("cmd:", msg.Get("action").String(), "data:", msg.Get("data").String())
				}

				if handler != nil {
					c := &Context{
						conn:  conn,
						house: house,
						data:  msg.Get("data"),
					}
					handler(c)
				} else {
					log.Printf("unhandled message: %s", message)
				}
			}()
		}
	})

	// 创建持久化房间
	for _, house := range base.Config.Persist {
		createHouse(house.ID, house.Name, house.Desc, house.Password, true)
	}

	// Snapshot 集成：启动时恢复，退出前保存
	snapshotURL := os.Getenv("SNAPSHOT_URL")
	var snapClient *snapshot.Client
	if snapshotURL != "" {
		snapClient = snapshot.NewClient(snapshotURL)
		if snapClient.Health() {
			log.Printf("snapshot service connected: %s", snapshotURL)
			restoreFromSnapshot(snapClient)
		} else {
			log.Printf("snapshot service not available: %s", snapshotURL)
		}
	}

	// 优雅退出：SIGINT/SIGTERM 时保存快照
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, saving snapshot...", sig)
		if snapClient != nil {
			dumpToSnapshot(snapClient)
		}
		os.Exit(0)
	}()

	// 定期保存快照（每 30 秒）
	if snapClient != nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				dumpToSnapshot(snapClient)
			}
		}()
	}

	if base.Config.Debug {
		log.Fatal(http.ListenAndServe(":8080", handler))
	} else {
		certmagic.HTTPS([]string{base.Config.Addr}, handler)
	}
}

var route = map[string]func(ctx *Context){
	"/chat":                 chat,
	"/setting/user":         setUser,
	"/setting/pull":         settingSync,
	"/music/search":         searchMusic,
	"/music/pick":           pickMusic,
	"/music/delete":         deleteMusic,
	"/music/good":           goodMusic,
	"/music/skip/vote":      voteSkip,
	"/music/searchsonglist": searchList,
	"/music/playmode":       playMode,
	"/music/sync":           getCurrentMusic,
	"/music/recommend":      recommendMusic,
	"/house/houseuser":      houseuser,
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "token,content-type,accesstoken")
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
	t := time.Now().Format(time.DateTime)
	fmt.Printf("[%s] %s \"%s %s\" %d\n", t, l.r.RemoteAddr, l.r.Method, l.r.URL.Path, statusCode)
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

// restoreFromSnapshot loads rooms from snapshot service and recreates them
func restoreFromSnapshot(client *snapshot.Client) {
	snap, err := client.Load()
	if err != nil {
		log.Printf("failed to load snapshot: %v", err)
		return
	}
	if snap == nil || len(snap.Rooms) == 0 {
		log.Println("no rooms to restore from snapshot")
		return
	}
	restored := 0
	for id, room := range snap.Rooms {
		// Skip if room already exists (from config persist)
		if GetHouse(id) != nil {
			continue
		}
		createHouse(id, room.Name, room.Desc, room.Password, true)
		// Restore playlist and mode
		h := GetHouse(id)
		if h == nil {
			continue
		}
		h.Mu.Lock()
		for _, s := range room.Playlist {
			h.Playlist = append(h.Playlist, Order{
				source: s.Source,
				id:     s.ID,
				user:   auth.User{Name: s.User},
				likes:  s.Likes,
			})
		}
		if room.Mode == "random" {
			h.Mode = RandomMode
		}
		h.Mu.Unlock()
		restored++
	}
	log.Printf("restored %d rooms from snapshot", restored)
}

// dumpToSnapshot collects all room states and saves to snapshot service
func dumpToSnapshot(client *snapshot.Client) {
	snap := snapshot.NewSnapshot()
	housesMu.Lock()
	for id, h := range houses {
		h.Mu.Lock()
		room := &snapshot.RoomState{
			ID:       id,
			Name:     h.Name,
			Desc:     h.Desc,
			Password: h.Password,
			Mode:     h.Mode.String(),
			PushTime: h.PushTime,
		}
		// Current song
		if h.Current.id != "" {
			room.Current = snapshot.Song{
				Source: h.Current.source,
				ID:     h.Current.id,
				User:   h.Current.user.Name,
				Likes:  h.Current.likes,
			}
		}
		// Playlist
		for _, o := range h.Playlist {
			room.Playlist = append(room.Playlist, snapshot.Song{
				Source: o.source,
				ID:     o.id,
				User:   o.user.Name,
				Likes:  o.likes,
			})
		}
		snap.Rooms[id] = room
		h.Mu.Unlock()
	}
	housesMu.Unlock()

	if err := client.SaveWithRetry(snap, 3); err != nil {
		log.Printf("failed to save snapshot: %v", err)
	} else {
		log.Printf("snapshot saved (%d rooms)", len(snap.Rooms))
	}
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
			hw:    w,
			house: house,
			data:  msg,
		}
		fn(ctx)
	}
}
