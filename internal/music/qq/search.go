package qq

import (
	"net/url"

	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/music/utils"
)

// Search 搜索歌曲，返回原始响应 gjson.Result（路径：data.song）
func (q *QQ) Search(keyword string) (gjson.Result, error) {
	params := url.Values{}
	params.Set("w", keyword)
	params.Set("format", "json")
	params.Set("p", "1")
	params.Set("n", "20")
	apiURL := "http://c.y.qq.com/soso/fcgi-bin/search_for_qq_cp?" + params.Encode()

	body, err := q.getCached(apiURL,
		utils.WithHeader("User-Agent", userAgent),
		utils.WithHeader("Referer", searchReferer),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return gjson.Result{}, err
	}
	return gjson.ParseBytes(body).Get("data.song"), nil
}

// SearchPlaylist 搜索歌单，返回原始响应 gjson.Result（路径：data）
func (q *QQ) SearchPlaylist(keyword string) (gjson.Result, error) {
	params := url.Values{}
	params.Set("query", keyword)
	params.Set("page_no", "0")
	params.Set("num_per_page", "20")
	params.Set("format", "json")
	params.Set("remoteplace", "txt.yqq.playlist")
	params.Set("flag_qc", "0")
	apiURL := "http://c.y.qq.com/soso/fcgi-bin/client_music_search_songlist?" + params.Encode()

	body, err := q.getCached(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/portal/search.html"),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return gjson.Result{}, err
	}
	return gjson.ParseBytes(unwrapJSONP(body)).Get("data"), nil
}

// GetSongDetail 获取歌曲详情，返回原始响应 gjson.Result（路径：data.0）
func (q *QQ) GetSongDetail(songMID string) (gjson.Result, error) {
	apiURL := "https://c.y.qq.com/v8/fcg-bin/fcg_play_single_song.fcg?songmid=" + songMID + "&format=json"
	body, err := q.getCached(apiURL,
		utils.WithHeader("User-Agent", userAgent),
		utils.WithHeader("Referer", searchReferer),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return gjson.Result{}, err
	}
	return gjson.ParseBytes(body).Get("data.0"), nil
}
