package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/syncx"
	"github.com/bihua-university/alisten/protocol"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

type Context struct {
	ctx     context.Context
	conn    *Connection
	hw      http.ResponseWriter
	house   *House
	data    gjson.Result
	message protocol.Incoming
	replied bool
}

// Decode reads the protocol DTO after the websocket envelope has been validated.
// HTTP callers share the DTOs while retaining their existing request wrapper.
func (c *Context) Decode(value any) bool {
	if err := json.Unmarshal([]byte(c.data.Raw), value); err != nil {
		c.Fail(protocol.ErrorCodeInvalidArgument, "请求数据格式错误")
		return false
	}
	return true
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
	if c.replied {
		return
	}
	c.replied = true
	if c.conn != nil {
		c.conn.Send(protocol.Response(c.message, j))
	}
	if c.hw != nil {
		writeJSON(c.hw, http.StatusOK, j)
	}
}

func (c *Context) Fail(code protocol.ErrorCode, message string) {
	if c.replied {
		return
	}
	c.replied = true
	if c.IsWebSocket() {
		c.conn.Send(protocol.Failure(c.message, protocol.Error{Code: code, Message: message}))
		return
	}
	status := http.StatusBadRequest
	switch code {
	case protocol.ErrorCodeNotFound:
		status = http.StatusNotFound
	case protocol.ErrorCodeRateLimited:
		status = http.StatusTooManyRequests
	case protocol.ErrorCodeUnavailable:
		status = http.StatusServiceUnavailable
	case protocol.ErrorCodeInternalError:
		status = http.StatusInternalServerError
	}
	writeJSON(c.hw, status, base.H{"error": message})
}

func (c *Context) Wait(kind uint8) bool {
	if !c.house.Wait(c.ctx, kind) {
		if c.ctx.Err() != nil {
			c.Fail(protocol.ErrorCodeUnavailable, "请求已取消或超时")
		} else {
			c.Fail(protocol.ErrorCodeRateLimited, "操作过于频繁，请稍后再试")
		}
		return false
	}
	return true
}

type Connection struct {
	ip        string
	send      syncx.UnboundedChan[[]byte]
	done      chan struct{}
	closeOnce sync.Once

	mu   sync.Mutex
	user auth.User

	conn *websocket.Conn
}

func (c *Connection) Start() {
	go func() {
		for x := range c.send.Out() {
			if err := c.conn.WriteMessage(websocket.TextMessage, x); err != nil {
				c.Close()
				return
			}
		}
	}()
}

func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		close(c.send.In())
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
}

func (c *Connection) GetUser() auth.User {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.user
}

func (c *Connection) Send(j any) {
	raw, err := json.Marshal(j)
	if err != nil {
		logx.Error("encode websocket message:", err)
		return
	}
	c.SendRaw(raw)
}

func (c *Connection) SendRaw(j []byte) {
	logx.Debug("[OUT]:", string(j))
	select {
	case <-c.done:
	case c.send.In() <- j:
	}
}
