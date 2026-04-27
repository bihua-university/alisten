package kuwo

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/bihua-university/alisten/internal/music/types"
	"github.com/bihua-university/alisten/internal/music/utils"
	"github.com/tidwall/gjson"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

type Kuwo struct{}

func New() *Kuwo {
	return &Kuwo{}
}

func (k *Kuwo) Search(keyword string) ([]types.Music, error) {
	params := url.Values{}
	params.Set("vipver", "1")
	params.Set("client", "kt")
	params.Set("ft", "music")
	params.Set("cluster", "0")
	params.Set("strategy", "2012")
	params.Set("encoding", "utf8")
	params.Set("rformat", "json")
	params.Set("mobi", "1")
	params.Set("issubtitle", "1")
	params.Set("show_copyright_off", "1")
	params.Set("pn", "0")
	params.Set("rn", "10")
	params.Set("all", keyword)

	apiURL := "http://www.kuwo.cn/search/searchMusicBykeyWord?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", userAgent),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	result := gjson.ParseBytes(body).Get("abslist")
	var songs []types.Music
	result.ForEach(func(_, item gjson.Result) bool {
		if item.Get("bitSwitch").Int() == 0 {
			return true
		}

		cleanID := strings.TrimPrefix(item.Get("MUSICRID").String(), "MUSIC_")
		duration, _ := strconv.ParseInt(item.Get("DURATION").String(), 10, 64)

		songs = append(songs, types.Music{
			ID:       cleanID,
			Name:     item.Get("SONGNAME").String(),
			Artist:   item.Get("ARTIST").String(),
			Album:    item.Get("ALBUM").String(),
			Duration: duration * 1000,
			Cover:    item.Get("hts_MVPIC").String(),
			Source:   types.KuWo,
		})
		return true
	})

	return songs, nil
}
