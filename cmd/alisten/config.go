package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bihua-university/alisten/internal/base"
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
	set, err := musicClient.CookieStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, base.H{"error": "音乐服务不可用"})
		return
	}
	writeJSON(w, http.StatusOK, base.H{"set": set})
}

// setCookieHTTP 更新音乐 Cookie：即时生效，并持久化到 config.json。
func setCookieHTTP(w http.ResponseWriter, r *http.Request) {
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

	persisted, err := musicClient.SetCookie(r.Context(), cookie)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, base.H{"error": "音乐服务不可用"})
		return
	}
	writeJSON(w, http.StatusOK, base.H{"message": "ok", "persisted": persisted})
}
