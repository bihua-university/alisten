package main

import (
	"fmt"
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

func (c *Context) Info(msg string) {
	h := base.H{
		"type": "info/push",
		"info": msg,
	}
	c.conn.Send(h)
}

func setUser(c *Context) {
	name := c.Get("name").String()
	email := c.Get("email").String()
	sendTime := c.Get("sendTime").Int()

	delay := sendTime - time.Now().UnixMilli()

	c.conn.mu.Lock()
	// 更新用户信息
	if email != "" {
		c.conn.user.Email = auth.EmailToMD5(email)
		c.conn.user.Name = name
	} else {
		c.conn.user.Name = fmt.Sprintf("%s(%s)", name, c.conn.ip)
		c.conn.user.Email = ""
	}
	c.conn.mu.Unlock()

	c.conn.Send(base.H{
		"type":  "delay",
		"delay": delay,
	})
	// 推送更新后的用户列表
	var users []auth.User
	c.WithHouse(func(h *House) {
		for _, conn := range h.Connection {
			users = append(users, conn.user)
		}
	})

	c.house.Broadcast(base.H{
		"type": "house_user",
		"data": users,
	})
}
