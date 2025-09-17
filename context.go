package main

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/syncx"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

type Context struct {
	conn  *Connection
	hw    http.ResponseWriter
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

func (c *Context) IsWebSocket() bool {
	return c.conn != nil
}

func (c *Context) IsHTTP() bool {
	return c.hw != nil
}

func (c *Context) User() auth.User {
	if c.IsWebSocket() {
		return c.conn.GetUser()
	}
	if c.IsHTTP() {
		email := c.Get("user.email").String()
		u := auth.User{
			Name:  c.Get("user.name").String(),
			Email: "",
		}
		if email != "" {
			u.Email = auth.EmailToMD5(email)
		}
		return u
	}
	return auth.User{}
}

func (c *Context) Send(j any) {
	if c.conn != nil {
		c.conn.Send(j)
	}
	if c.hw != nil {
		writeJSON(c.hw, http.StatusOK, j)
	}
}

func (c *Context) Broadcast(j any) {
	if c.conn != nil {
		c.house.Broadcast(j)
	}
	if c.hw != nil {
		writeJSON(c.hw, http.StatusOK, j)
	}
}

type Connection struct {
	ip   string
	send syncx.UnboundedChan[[]byte]

	mu   sync.Mutex
	user auth.User

	conn *websocket.Conn
}

func (c *Connection) Start() {
	go func() {
		for x := range c.send.Out() {
			_ = c.conn.WriteMessage(websocket.TextMessage, x)
		}
	}()
}

func (c *Connection) GetUser() auth.User {
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
