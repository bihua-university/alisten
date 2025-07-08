package main

import (
	"time"
)

func chat(c *Context) {
	// 转发所有消息
	msg := H{
		"type":     "chat",
		"nickName": c.conn.GetUser(),
		"sendTime": c.data.Get("sendTime").Int(),
		"content":  c.data.Get("content").String(),
	}
	c.house.Broadcast(msg)
}

func (c *Context) Chat(msg string) {
	h := H{
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
	c.conn.Send(H{
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
	c.house.Broadcast(H{
		"type": "house_user",
		"data": u,
	})
}
