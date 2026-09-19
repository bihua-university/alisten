package base

import (
	"encoding/json"
	"os"
)

type H = map[string]any

var Config struct {
	Addr       string
	Token      string
	Cookie     string
	NeteaseAPI string
	QQAPI      string
	Pgsql      string
	Debug      bool
	Persist    []PersistHouse
}

// configFile 对应 config.json 的原始结构，music 字段平铺到 Config。
type configFile struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
	Music struct {
		Cookie  string `json:"cookie"`
		Netease string `json:"netease"`
		QQ      string `json:"qq"`
	} `json:"music"`
	Pgsql   string         `json:"pgsql"`
	Debug   bool           `json:"debug"`
	Persist []PersistHouse `json:"persist"`
}

type PersistHouse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Password string `json:"password"`
}

func InitConfig() {
	file, _ := os.ReadFile("config.json")

	var c configFile
	if err := json.Unmarshal(file, &c); err != nil {
		return // 配置无效时保持零值，与历史行为一致
	}

	Config.Addr = c.Addr
	Config.Token = c.Token
	Config.Cookie = c.Music.Cookie
	Config.NeteaseAPI = c.Music.Netease
	Config.QQAPI = c.Music.QQ
	Config.Pgsql = c.Pgsql
	Config.Debug = c.Debug
	Config.Persist = c.Persist
}

// SetCookie 更新内存中的音乐 Cookie 并持久化到 config.json。
func SetCookie(cookie string) error {
	Config.Cookie = cookie

	data, err := os.ReadFile("config.json")
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	music, _ := m["music"].(map[string]any)
	if music == nil {
		music = map[string]any{}
		m["music"] = music
	}
	music["cookie"] = cookie

	out, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile("config.json", out, 0o600)
}
