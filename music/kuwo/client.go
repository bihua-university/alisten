package kuwo

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
)

type Client struct {
	ArrayMask [64]int64
}

type KWSearchResp struct {
	Abslist []Abslist `json:"abslist"`
}

type Abslist struct {
	MUSICRID string `json:"MUSICRID"`
}

// NewClient 创建一个kuwo客户端
func NewClient() *Client {
	var client = &Client{}
	for i := 0; i < 63; i++ {
		client.ArrayMask[i] = int64(math.Pow(float64(2), float64(i)))
	}
	client.ArrayMask[len(client.ArrayMask)-1] = -9223372036854775808
	return client
}
func KuWwSearchApi(pageNo, pageSize int, key string) string {
	escape := url.QueryEscape(key)
	return fmt.Sprintf(KWSearchApi, pageNo, pageSize, escape)
}

// SearchMusic 搜索音乐(实现musicClient interface)
func (k *Client) SearchMusic(pageNo, pageSize int, kw string) *KWSearchResp {
	api := KuWwSearchApi(pageNo, pageSize, kw)
	resp := HttpGetWithHeader(api, KWSearchHead)
	if resp.Err != nil {
		panic(resp.Err)
	}
	kwResp := new(KWSearchResp)
	err := json.Unmarshal(resp.Data, kwResp)
	if err != nil {
		panic(err)
	}
	return kwResp
}

const KWSearchApi = "https://search.kuwo.cn/r.s?pn=%d&rn=%d&all=%s&ft=music&newsearch=1&alflac=1&itemset=web_2013&client=kt&cluster=0&vermerge=1&rformat=json&encoding=utf8&show_copyright_off=1&pcmp4=1&ver=mbox&plat=pc&vipver=1&devid=11404450&newver=1&issubtitle=1&pcjson=1"

var (
	KWSearchHead = map[string]string{
		"user_agent": `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36 Edg/110.0.1587.50`,
		"accept":     `text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7`,
		"referer":    `http://kuwo.cn/search/list?key=%E6%81%90%E9%BE%99%E6%8A%97%E7%8B%BC8`,
		"Secret":     "13261c0dccfeac48dd7a8b33de9fd1bb59e7bcd1fbda77ed3a2e42bce5fc7e0f0036d507",
		"Cross":      "e5191b2eb629a3da9dc6868755a3e779",
		"Cookie":     "ga=GA1.2.1860922824.1635265329; Hm_lvt_cdb524f42f0ce19b169a8071123a4797=1663159268; gid=9ed7ed0b-8d4b-4167-8c9d-f1f2c55642f7; Hm_token=et7csP3xeQfeadZsDEazXEpEXhmjTC4k; Hm_Iuvt_cdb524f42f0ce19b169b8072123a4727=Mzfa6zAAcAfszyHFdREYF7KfBRNmAEi4",
	}
)
