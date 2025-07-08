package base

import (
	"os"
	"reflect"

	"github.com/tidwall/gjson"
)

type H = map[string]any

var Config struct {
	Addr       string         `config:"addr"`
	Token      string         `config:"token"`
	Cookie     string         `config:"music.cookie"`
	NeteaseAPI string         `config:"music.netease"`
	QQAPI      string         `config:"music.qq"`
	Pgsql      string         `config:"pgsql"`
	Debug      bool           `config:"debug"`
	Persist    []PersistHouse `config:"persist"`
}

type PersistHouse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	Password string `json:"password"`
}

func InitConfig() {
	file, _ := os.ReadFile("config.json")
	g := gjson.Parse(string(file))

	var (
		v                     = reflect.ValueOf(&Config).Elem()
		t                     = v.Type()
		stringType            = reflect.TypeOf("")
		boolType              = reflect.TypeOf(true)
		slicePersistHouseType = reflect.TypeOf([]PersistHouse{})
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
		case slicePersistHouseType:
			// 处理 persist 字段
			persistData := g.Get(name)
			if persistData.Exists() && persistData.IsArray() {
				var houses []PersistHouse
				persistData.ForEach(func(key, value gjson.Result) bool {
					house := PersistHouse{
						ID:       value.Get("id").String(),
						Name:     value.Get("name").String(),
						Desc:     value.Get("desc").String(),
						Password: value.Get("password").String(),
					}
					houses = append(houses, house)
					return true
				})
				v.Field(i).Set(reflect.ValueOf(houses))
			}
		default:
			panic("unsupported type")
		}
	}
}
