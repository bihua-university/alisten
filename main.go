package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/base"
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
	"/music/playlist":       getPlaylist,
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
