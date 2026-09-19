package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/logx"
	"github.com/bihua-university/alisten/internal/mpipe"
	"github.com/bihua-university/alisten/internal/music"
)

// authConfig 校验管理接口的 Bearer Token（即 config.json 中的 token）。
func authConfig(w http.ResponseWriter, r *http.Request) bool {
	if base.Config.Token == "" {
		writeJSON(w, http.StatusForbidden, base.H{"error": "服务端未配置 token，已拒绝访问"})
		return false
	}
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token != base.Config.Token {
		writeJSON(w, http.StatusUnauthorized, base.H{"error": "未授权"})
		return false
	}
	return true
}

// getCookieHTTP 返回当前音乐 Cookie 是否已设置（不泄露具体值）。
func getCookieHTTP(w http.ResponseWriter, r *http.Request) {
	if !authConfig(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, base.H{"set": base.Config.Cookie != ""})
}

// setCookieHTTP 更新音乐 Cookie：即时生效，并持久化到 config.json。
func setCookieHTTP(w http.ResponseWriter, r *http.Request, pipe *mpipe.Pipe) {
	if !authConfig(w, r) {
		return
	}

	var request struct {
		Cookie string `json:"cookie"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, base.H{"error": "无效的JSON格式"})
		return
	}
	cookie := strings.TrimSpace(request.Cookie)
	if cookie == "" {
		writeJSON(w, http.StatusBadRequest, base.H{"error": "cookie 不能为空"})
		return
	}

	// 更新所有使用 Cookie 的组件
	music.SetNeteaseCookie(cookie)
	pipe.SetCookie(cookie)

	persisted := true
	if err := base.SetCookie(cookie); err != nil {
		logx.Error("persist cookie:", err)
		persisted = false
	}
	writeJSON(w, http.StatusOK, base.H{"message": "ok", "persisted": persisted})
}
