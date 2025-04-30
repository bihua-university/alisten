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
}

func InitConfig() {
	file, _ := os.ReadFile("config.json")
	g := gjson.Parse(string(file))

	var (
		v          = reflect.ValueOf(&Config).Elem()
		t          = v.Type()
		stringType = reflect.TypeOf("")
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
		default:
			panic("unsupported type")
		}
	}
}
