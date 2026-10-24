package musicbackend

import (
	"context"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/bihua-university/alisten/internal/music"
	pb "github.com/bihua-university/alisten/internal/proto/musicd"
)

var cache = expirable.NewLRU[string, *pb.Song](512, nil, 30*time.Minute)

// GetMusic 获取一首歌的完整播放信息，带缓存。
// useCache 为 false 时，网易云（链接有时效性）会强制刷新，其余来源仍优先读缓存。
func GetMusic(ctx context.Context, source, id string, useCache bool) (*pb.Song, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	src := music.ParseSource(source)

	key := source + "OvO" + id
	if useCache || src != music.NetEase {
		if v, ok := cache.Get(key); ok {
			return v, nil
		}
	}

	p, err := providerFor(src)
	if err != nil {
		return nil, err
	}
	song, err := p.GetSong(ctx, id)
	if err != nil {
		return nil, err
	}

	cache.Add(key, song)
	return song, nil
}
