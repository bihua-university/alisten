package base

import (
	"os"
	"reflect"

	"github.com/tidwall/gjson"
)

var Config struct {
	Addr       string `config:"addr"`
	Cookie     string `config:"music.cookie"`
	NeteaseAPI string `config:"music.netease"`
	QQAPI      string `config:"music.qq"`
	QiniuAK    string `config:"qiniu.ak"`
	QiniuSK    string `config:"qiniu.sk"`
	Pgsql      string `config:"pgsql"`
	Debug      bool   `config:"debug"`
}

func InitConfig() {
	file, _ := os.ReadFile("config.json")
	g := gjson.Parse(string(file))

	var (
		v          = reflect.ValueOf(&Config).Elem()
		t          = v.Type()
		stringType = reflect.TypeOf("")
		boolType   = reflect.TypeOf(true)
	)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := field.Tag.Get("config")
		if name == "" {
			continue
		}
		switch field.Type {
		case stringType:
			v.Field(i).SetString(g.Get(name).String())
		case boolType:
			v.Field(i).SetBool(g.Get(name).Bool())
		default:
			panic("unsupported type")
		}
	}
}
