package qq

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/bihua-university/alisten/internal/music/internal/types"
)

// Search 搜索歌曲，返回全量结果（分页由上层处理）
func (c *Client) Search(keyword string) (*types.Result[types.Music], error) {
	params := url.Values{}
	params.Set("w", keyword)
	params.Set("format", "json")
	params.Set("p", "1")
	params.Set("n", "20")
	apiURL := "http://c.y.qq.com/soso/fcgi-bin/search_for_qq_cp?" + params.Encode()

	body, err := c.getCached(apiURL, http.Header{
		"User-Agent": {userAgent},
		"Referer":    {searchReferer},
	})
	if err != nil {
		return nil, err
	}

	var data []*types.Music
	gjson.ParseBytes(body).Get("data.song.list").ForEach(func(_, item gjson.Result) bool {
		data = append(data, &types.Music{
			ID:       item.Get("songmid").String(),
			Name:     item.Get("songname").String(),
			Artist:   joinSingers(item.Get("singer")),
			Album:    item.Get("albumname").String(),
			Duration: item.Get("interval").Int() * 1000,
			Cover:    coverURL(item.Get("albummid").String()),
			Source:   types.QQ,
		})
		return true
	})
	return &types.Result[types.Music]{Total: int64(len(data)), Data: data}, nil
}

// SearchPlaylist 搜索歌单，返回全量结果（分页由上层处理）
func (c *Client) SearchPlaylist(keyword string) (*types.Result[types.Playlist], error) {
	params := url.Values{}
	params.Set("query", keyword)
	params.Set("page_no", "0")
	params.Set("num_per_page", "20")
	params.Set("format", "json")
	params.Set("remoteplace", "txt.yqq.playlist")
	params.Set("flag_qc", "0")
	apiURL := "http://c.y.qq.com/soso/fcgi-bin/client_music_search_songlist?" + params.Encode()

	body, err := c.getCached(apiURL, http.Header{
		"User-Agent": {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"},
		"Referer":    {"https://y.qq.com/portal/search.html"},
	})
	if err != nil {
		return nil, err
	}

	var data []*types.Playlist
	gjson.ParseBytes(unwrapJSONP(body)).Get("data.list").ForEach(func(_, item gjson.Result) bool {
		cover := item.Get("imgurl").String()
		if strings.HasPrefix(cover, "http://") {
			cover = strings.Replace(cover, "http://", "https://", 1)
		}
		data = append(data, &types.Playlist{
			ID:         item.Get("dissid").String(),
			Name:       item.Get("dissname").String(),
			PictureURL: cover,
			Desc:       item.Get("introduction").String(),
			Creator:    item.Get("creator.name").String(),
			CreatorUid: item.Get("creator.creator_uin").String(),
			PlayCount:  item.Get("listennum").Int(),
			SongCount:  item.Get("song_count").Int(),
		})
		return true
	})
	return &types.Result[types.Playlist]{Total: int64(len(data)), Data: data}, nil
}

// GetSongDetail 获取歌曲详情
func (c *Client) GetSongDetail(songMID string) (*types.Music, error) {
	apiURL := "https://c.y.qq.com/v8/fcg-bin/fcg_play_single_song.fcg?songmid=" + songMID + "&format=json"
	body, err := c.getCached(apiURL, http.Header{
		"User-Agent": {userAgent},
		"Referer":    {searchReferer},
	})
	if err != nil {
		return nil, err
	}

	song := gjson.ParseBytes(body).Get("data.0")
	if !song.Exists() {
		return nil, errors.New("song not found")
	}
	return &types.Music{
		ID:       songMID,
		Name:     song.Get("name").String(),
		Artist:   joinSingers(song.Get("singer")),
		Album:    song.Get("album.name").String(),
		Duration: song.Get("interval").Int() * 1000,
		Cover:    coverURL(song.Get("album.mid").String()),
		Source:   types.QQ,
	}, nil
}
