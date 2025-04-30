package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/wdvxdr1123/alisten/internal/base"
	"github.com/wdvxdr1123/alisten/internal/syncx"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

//go:embed dist
var fronted embed.FS

const Debug = true

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
} // use default options

func main() {
	base.InitConfig()

	gin.SetMode(gin.ReleaseMode)
	// 房间相关路由
	g := gin.Default()
	g.Use(Cors())
	g.Any("/house/add", addHouse)
	g.Any("/house/enter", enterHouse)
	g.Any("/house/search", searchHouses)

	f, _ := fs.Sub(fronted, "dist")
	g.StaticFS("/listen", http.FS(f))

	g.Any("/server", func(c *gin.Context) {
		w, r := c.Writer, c.Request
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "token,content-type,accesstoken")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

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
			user: "(" + ip + ")",
			send: syncx.NewUnboundedChan[[]byte](8),
		}

		house.Mu.Lock()
		house.Connection = append(house.Connection, conn)
		house.Mu.Unlock()

		conn.Start()
		house.enter(conn)

		for {
			_, message, err := wc.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				// remove from connections
				house.lock(func() {
					for i, conn := range house.Connection {
						if conn.conn == wc {
							house.Connection = append(house.Connection[:i], house.Connection[i+1:]...)
						}
					}
				})
				break
			}

			// async handle command
			go func() {
				msg := gjson.ParseBytes(message)
				handler := route[msg.Get("action").String()]

				if Debug {
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

	log.Fatal(http.ListenAndServe(base.Config.Addr, g))
}

var route = map[string]func(ctx *Context){
	"/chat":                 chat,
	"/setting/name":         setName,
	"/music/search":         searchMusic,
	"/music/pick":           pickMusic,
	"/music/delete":         deleteMusic,
	"/music/good":           goodMusic,
	"/music/skip/vote":      voteSkip,
	"/music/searchsonglist": searchList,
	"/house/houseuser":      houseuser,
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "token,content-type,accesstoken")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
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
