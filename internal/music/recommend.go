package music

import (
	"slices"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// Recommender 基于网易云相似歌曲接口做听歌推荐。
type Recommender struct {
	mu        sync.Mutex
	history   []string                        // music id
	recommend []string                        // recommended music id
	cache     expirable.LRU[string, []string] // cache recommended music for playlist
}

const maxHistorySize = 32

func NewRecommender() *Recommender {
	return &Recommender{
		cache: *expirable.NewLRU[string, []string](128, nil, 30*time.Minute), // 30 minutes
	}
}

// AddHistory 添加音乐到历史记录，保持不超过 maxHistorySize
//
// TODO(wdvxdr1123): use ring buffer to optimize memory usage
func (mr *Recommender) AddHistory(m string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	if mr.history == nil {
		mr.history = make([]string, 0, maxHistorySize)
	}
	if m == "" {
		return
	}
	// 检查是否已存在
	if slices.Contains(mr.history, m) {
		return
	}
	// 添加新音乐
	if len(mr.history) < maxHistorySize {
		mr.history = append(mr.history, m)
	} else {
		// 移除最早的音乐，添加新音乐
		mr.history = append(mr.history[1:], m)
	}
}

func (mr *Recommender) Mark(m string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	if mr.recommend == nil {
		mr.recommend = make([]string, 0, maxHistorySize)
	}
	if m == "" {
		return
	}
	// 检查是否已存在
	if slices.Contains(mr.recommend, m) {
		return
	}
	// 添加新音乐
	if len(mr.recommend) < maxHistorySize*4 {
		mr.recommend = append(mr.recommend, m)
	} else {
		// 移除最早的音乐，添加新音乐
		mr.recommend = append(mr.recommend[1:], m)
	}
}

func (mr *Recommender) Enabled() bool {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	return len(mr.history) >= 3
}

// Recommend 返回推荐的音乐id列表，按照添加顺序返回
func (mr *Recommender) Recommend(playlist []string) []string {
	// we use map to deduplicate music and random access

	mr.mu.Lock()
	list := make(map[string]struct{})
	visit := make(map[string]struct{})
	for _, m := range mr.history {
		list[m] = struct{}{}
		visit[m] = struct{}{}
	}
	for _, m := range mr.recommend {
		// don't using recommended music to recommend again
		visit[m] = struct{}{}
	}
	mr.mu.Unlock()
	// also mark current playlist as visited
	for _, id := range playlist {
		list[id] = struct{}{}
		visit[id] = struct{}{}
	}

	item := make(map[string]struct{})
	push := func(id string) {
		if r, ok := mr.cache.Get(id); ok {
			for _, v := range r {
				if _, ok := visit[v]; !ok { // don't recommend existed music
					item[v] = struct{}{}
				}
			}
			return
		}

		recommend, err := neteaseAPI.GetSimilarSongs(id)
		if err != nil {
			return
		}
		for _, sid := range recommend {
			if _, ok := visit[sid]; !ok { // don't recommend existed music
				item[sid] = struct{}{}
			}
		}
		mr.cache.Add(id, recommend)
	}

	for v := range list {
		if len(item) > 25 { // only recommend based on 25 music
			break
		}
		push(v)
	}

	result := make([]string, 0, len(item))
	for id := range item {
		result = append(result, id)
	}
	return result[0:min(len(result), 10)] // at most 10 music
}
