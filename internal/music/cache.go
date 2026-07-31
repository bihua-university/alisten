package music

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/bihua-university/alisten/internal/music/internal/types"
)

var cache = expirable.NewLRU[string, *Song](512, nil, 30*time.Minute)

// GetMusic 获取一首歌的完整播放信息，带缓存。
// useCache 为 false 时，网易云（链接有时效性）会强制刷新，其余来源仍优先读缓存。
func GetMusic(source, id string, useCache bool) (*Song, error) {
	src := types.ParseSource(source)

	key := source + "OvO" + id
	if useCache || src != types.NetEase {
		if v, ok := cache.Get(key); ok {
			return v, nil
		}
	}

	p, err := providerFor(src)
	if err != nil {
		return nil, err
	}
	song, err := p.GetSong(id)
	if err != nil {
		return nil, err
	}

	cache.Add(key, song)
	return song, nil
}
