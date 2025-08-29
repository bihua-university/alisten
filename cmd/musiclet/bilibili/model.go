package bilibili

import (
	"io"
)

const (
	WebInterfaceView = "https://api.bilibili.com/x/web-interface/view"
	PlayerPlayUrl    = "https://api.bilibili.com/x/player/playurl"
)

var SessionData = ""

type Resp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Ttl     int    `json:"ttl"`
}

type Response[RespData WebInterfaceViewRespData | VideoPlayRespData] struct {
	Resp
	Data RespData `json:"data"`
}

type WebInterfaceViewRespData struct {
	Aid      int    `json:"aid"`
	Cid      int    `json:"cid"`
	Pic      string `json:"pic"`
	Title    string `json:"title"`
	Duration int    `json:"duration"`
	Owner    Owner  `json:"owner"`
}

type Owner struct {
	Name string `json:"name"`
}

type VideoPlayRespData struct {
	Dash VideoDash `json:"dash"`
}

type VideoDash struct {
	Audio []AudioStream `json:"audio"`
}

type AudioStream struct {
	ID      int    `json:"id"`
	BaseURL string `json:"baseUrl"`
}

type Downloader struct {
	io.Reader
	Total   int64
	Current int64
}

func (d *Downloader) Read(p []byte) (n int, err error) {
	n, err = d.Reader.Read(p)
	d.Current += int64(n)
	return
}
