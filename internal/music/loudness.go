package music

import (
	"fmt"
	"net/http"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"golang.org/x/sync/singleflight"

	"github.com/bihua-university/alisten/internal/music/loudness"
)

// loudnessCache 缓存响度分析结果;结果不会变化,只做 LRU 淘汰。
var loudnessCache = mustLRU[string, *loudness.Result](1024)

func mustLRU[K comparable, V any](size int) *lru.Cache[K, V] {
	c, err := lru.New[K, V](size)
	if err != nil {
		panic(err)
	}
	return c
}

func loudnessKey(source, id string) string {
	return source + "OvO" + id
}

// GetCachedLoudness 读取缓存的分析结果;分析失败的歌曲缓存为 nil (ok 为 true)。
func GetCachedLoudness(source, id string) (*loudness.Result, bool) {
	return loudnessCache.Get(loudnessKey(source, id))
}

func SetCachedLoudness(source, id string, r *loudness.Result) {
	loudnessCache.Add(loudnessKey(source, id), r)
}

// GetLoudnessCoefficient 返回响度归一化的线性音量系数 (Spotify 策略)。
// 保证返回时分析已完成: 未缓存则同步分析, 分析失败返回 1 (不调整)。
func GetLoudnessCoefficient(source, id string) float64 {
	analyzeOnce(source, id)
	if r, ok := GetCachedLoudness(source, id); ok && r != nil {
		return r.Coefficient()
	}
	return 1
}

var analyzeHTTPClient = &http.Client{Timeout: 2 * time.Minute}

// analyzeSema 限制并发分析数,避免下载/解码打满 CPU 与带宽。
var analyzeSema = make(chan struct{}, 4)

// analyzeFunc 实际执行分析,测试可注入假实现;失败时缓存 nil,避免反复重试下载。
var analyzeFunc = func(source, id string) {
	if _, err := AnalyzeLoudness(source, id); err != nil {
		SetCachedLoudness(source, id, nil)
	}
}

// analyzeGroup 对同一首歌的并发分析去重,等待方共享同一次分析。
var analyzeGroup singleflight.Group

// analyzeOnce 确保分析完成: 已缓存直接返回, 否则执行或等待同 key 的分析。
func analyzeOnce(source, id string) {
	if _, ok := GetCachedLoudness(source, id); ok {
		return
	}
	analyzeGroup.Do(loudnessKey(source, id), func() (any, error) {
		if _, ok := GetCachedLoudness(source, id); ok { // 等待期间可能已完成
			return nil, nil
		}
		analyzeSema <- struct{}{}
		defer func() { <-analyzeSema }()
		analyzeFunc(source, id)
		return nil, nil
	})
}

// AnalyzeLoudnessAsync 后台分析歌曲响度并缓存结果。
func AnalyzeLoudnessAsync(source, id string) {
	go analyzeOnce(source, id)
}

// AnalyzeLoudness 下载歌曲音频并做 BS.1770 响度分析,结果按 source+id 缓存。
func AnalyzeLoudness(source, id string) (*loudness.Result, error) {
	if r, ok := GetCachedLoudness(source, id); ok {
		if r == nil {
			return nil, fmt.Errorf("source %s id %s: 此前的响度分析失败", source, id)
		}
		return r, nil
	}

	m, err := GetMusic(source, id, true)
	if err != nil {
		return nil, fmt.Errorf("获取歌曲信息失败: %w", err)
	}
	if m.URL == "" {
		return nil, fmt.Errorf("source %s id %s: 没有可下载的播放地址", source, id)
	}

	resp, err := analyzeHTTPClient.Get(m.URL)
	if err != nil {
		return nil, fmt.Errorf("下载音频失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载音频失败: HTTP %d", resp.StatusCode)
	}

	r, err := loudness.AnalyzeReader(resp.Body)
	if err != nil {
		return nil, err
	}

	SetCachedLoudness(source, id, r)
	return r, nil
}
