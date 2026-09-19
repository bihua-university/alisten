package base

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInitConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	content := `{
		"addr": ":8080",
		"token": "test-token",
		"music": {
			"netease": "http://localhost:3000",
			"cookie": "MUSIC_U=abc",
			"qq": "http://localhost:3300"
		},
		"debug": true,
		"pgsql": "host=localhost",
		"persist": [{"id": "room", "name": "音乐房间", "desc": "d", "password": "pw"}]
	}`
	if err := os.WriteFile("config.json", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	InitConfig()

	if Config.Addr != ":8080" || Config.Token != "test-token" {
		t.Errorf("unexpected addr/token: %q %q", Config.Addr, Config.Token)
	}
	if Config.Cookie != "MUSIC_U=abc" || Config.NeteaseAPI != "http://localhost:3000" || Config.QQAPI != "http://localhost:3300" {
		t.Errorf("unexpected music config: %+v", Config)
	}
	if !Config.Debug || Config.Pgsql != "host=localhost" {
		t.Errorf("unexpected debug/pgsql: %+v", Config)
	}
	if len(Config.Persist) != 1 || Config.Persist[0].ID != "room" || Config.Persist[0].Password != "pw" {
		t.Errorf("unexpected persist: %+v", Config.Persist)
	}
}

func TestInitConfigInvalidJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.json", []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	// 无效配置不应 panic（Config 为全局变量，字段保持原值）
	InitConfig()
}

func TestSetCookie(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	content := `{"addr": ":8080", "music": {"cookie": "old", "netease": "http://x"}}`
	if err := os.WriteFile("config.json", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := SetCookie("new-cookie"); err != nil {
		t.Fatal(err)
	}
	if Config.Cookie != "new-cookie" {
		t.Errorf("memory not updated: %q", Config.Cookie)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	music := m["music"].(map[string]any)
	if music["cookie"] != "new-cookie" || music["netease"] != "http://x" {
		t.Errorf("persist failed: %s", data)
	}
	if m["addr"] != ":8080" {
		t.Errorf("other field lost: %s", data)
	}
}
