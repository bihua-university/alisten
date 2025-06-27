package main

import (
	"time"

	"github.com/gin-gonic/gin"
)

func chat(c *Context) {
	// 转发所有消息
	msg := gin.H{
		"type":     "chat",
		"nickName": c.conn.GetUser(),
		"sendTime": c.data.Get("sendTime").Int(),
		"content":  c.data.Get("content").String(),
	}
	c.house.Broadcast(msg)
}

func (c *Context) Chat(msg string) {
	h := gin.H{
		"type":     "chat",
		"nickName": c.conn.GetUser(),
		"sendTime": time.Now().UnixMilli(),
		"content":  msg,
	}
	c.house.Broadcast(h)
}

func setName(c *Context) {
	c.conn.mu.Lock()
	defer c.conn.mu.Unlock()
	delay := c.Get("sendTime").Int() - time.Now().UnixMilli()
	c.conn.user = c.data.Get("name").String() + "(" + c.conn.ip + ")"
	c.conn.Send(gin.H{
		"type":  "delay",
		"delay": delay,
	})

	// 推送更新后的用户列表
	var u []string
	c.WithHouse(func(h *House) {
		for _, conn := range h.Connection {
			u = append(u, conn.user)
		}
	})
	c.house.Broadcast(gin.H{
		"type": "house_user",
		"data": u,
	})
}
