package main

import (
	"fmt"
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/protocol"
)

func chat(c *Context) {
	var request protocol.ChatRequest
	if !c.Decode(&request) {
		return
	}
	c.Chat(request.Content)
	c.Send(protocol.Empty{})
}

func (c *Context) Chat(content string) {
	if chars := []rune(content); len(chars) > 500 {
		content = string(chars[:500])
	}
	c.house.Broadcast(protocol.Event(protocol.EventChatMessage, protocol.ChatData{
		Content: content, User: protocol.UserData(c.User()), SentAtMS: time.Now().UnixMilli(),
	}))
}

func setUser(c *Context) {
	var request protocol.UserUpdateRequest
	if !c.Decode(&request) {
		return
	}
	name := request.Name
	user := auth.User{Name: name}
	if email := request.Email; email != "" {
		user.Email = auth.EmailToMD5(email)
	} else {
		user.Name = fmt.Sprintf("%s(%s)", name, c.conn.ip)
	}
	c.conn.mu.Lock()
	c.conn.user = user
	c.conn.mu.Unlock()
	c.house.PushMembers()
	c.Send(protocol.UserUpdateResult{
		User: protocol.UserData(user), ClientTimeMS: request.ClientTimeMS, ServerTimeMS: time.Now().UnixMilli(),
	})
}

func heartbeat(c *Context) {
	var request protocol.HeartbeatRequest
	if c.Decode(&request) {
		c.Send(protocol.HeartbeatResult{ClientTimeMS: request.ClientTimeMS, ServerTimeMS: time.Now().UnixMilli()})
	}
}
