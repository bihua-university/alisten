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
	MusicRPC   MusicRPCConfig
}

type MusicRPCConfig struct {
	Address        string `json:"address"`
	HTTPURL        string `json:"http_url"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

var configPath = "config.json"

// configFile 对应 config.json 的原始结构，music 字段平铺到 Config。
type configFile struct {
	Addr  string `json:"addr"`
	Token string `json:"token"`
	Music struct {
		Cookie  string         `json:"cookie"`
		Netease string         `json:"netease"`
		QQ      string         `json:"qq"`
		RPC     MusicRPCConfig `json:"rpc"`
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
	_ = LoadConfig("config.json")
}

// LoadConfig also selects the file used by runtime Cookie persistence.
func LoadConfig(path string) error {
	file, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var c configFile
	if err := json.Unmarshal(file, &c); err != nil {
		return err
	}
	configPath = path

	Config.Addr = c.Addr
	Config.Token = c.Token
	Config.Cookie = c.Music.Cookie
	Config.NeteaseAPI = c.Music.Netease
	Config.QQAPI = c.Music.QQ
	Config.Pgsql = c.Pgsql
	Config.Debug = c.Debug
	Config.Persist = c.Persist
	Config.MusicRPC = c.Music.RPC
	return nil
}

// SetCookie 更新内存中的音乐 Cookie 并持久化到加载的配置文件。
func SetCookie(cookie string) error {
	Config.Cookie = cookie

	data, err := os.ReadFile(configPath)
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
	return os.WriteFile(configPath, out, 0o600)
}
