package main

import (
	"encoding/json"
	"sync"

	"github.com/bihua-university/alisten/internal/syncx"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

type Context struct {
	conn  *Connection
	house *House
	data  gjson.Result
}

func (c *Context) Get(p string) gjson.Result {
	return c.data.Get(p)
}

func (c *Context) WithHouse(f func(*House)) {
	c.house.Mu.Lock()
	defer c.house.Mu.Unlock()
	f(c.house)
}

type Connection struct {
	ip   string
	send syncx.UnboundedChan[[]byte]

	mu   sync.Mutex
	user string

	conn *websocket.Conn
}

func (c *Connection) Start() {
	go func() {
		for x := range c.send.Out() {
			_ = c.conn.WriteMessage(websocket.TextMessage, x)
		}
	}()
}

func (c *Connection) GetUser() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.user
}

func encJson(j any) []byte {
	b, _ := json.Marshal(j)
	return b
}

func (c *Connection) Send(j any) {
	c.SendRaw(encJson(j))
}

func (c *Connection) SendRaw(j []byte) {
	c.send.In() <- j
}
