package main

import (
	"time"

	"github.com/bihua-university/alisten/internal/auth"
	"github.com/bihua-university/alisten/internal/base"
)

func chat(c *Context) {
	// 转发所有消息
	msg := base.H{
		"type":     "chat",
		"user":     c.conn.GetUser(),
		"sendTime": c.data.Get("sendTime").Int(),
		"content":  c.data.Get("content").String(),
	}
	c.house.Broadcast(msg)
}

func (c *Context) Chat(msg string) {
	h := base.H{
		"type":     "chat",
		"user":     c.conn.GetUser(),
		"sendTime": time.Now().UnixMilli(),
		"content":  msg,
	}
	c.house.Broadcast(h)
}

func setUser(c *Context) {
	c.conn.mu.Lock()
	defer c.conn.mu.Unlock()
	delay := c.Get("sendTime").Int() - time.Now().UnixMilli()
	c.conn.user.Name = c.data.Get("name").String()
	c.conn.user.Email = c.data.Get("email").String()
	if c.conn.user.Email == "" {
		c.conn.user.Name = c.conn.user.Name + "(" + c.conn.ip + ")"
	}
	c.conn.Send(base.H{
		"type":  "delay",
		"delay": delay,
	})

	// 推送更新后的用户列表
	var u []auth.User
	c.WithHouse(func(h *House) {
		for _, conn := range h.Connection {
			u = append(u, conn.user)
		}
	})
	c.house.Broadcast(base.H{
		"type": "house_user",
		"data": u,
	})
}
