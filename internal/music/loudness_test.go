package music

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bihua-university/alisten/internal/music/loudness"
)

func TestLoudnessCache(t *testing.T) {
	r := &loudness.Result{Integrated: -12.5, Peak: -1.2}
	SetCachedLoudness("netease", "12345", r)

	got, ok := GetCachedLoudness("netease", "12345")
	if !ok || got != r {
		t.Fatalf("缓存命中失败: ok=%v got=%v", ok, got)
	}

	if _, ok := GetCachedLoudness("qq", "12345"); ok {
		t.Error("不同 source 不应命中同一缓存")
	}
	if _, ok := GetCachedLoudness("netease", "99999"); ok {
		t.Error("不同 id 不应命中同一缓存")
	}
}

func TestGetLoudnessCoefficient(t *testing.T) {
	SetCachedLoudness("coef", "1", &loudness.Result{Integrated: -20, Peak: -10})
	// -14 - (-20) = +6 dB -> 系数约 1.995
	if c := GetLoudnessCoefficient("coef", "1"); c < 1.99 || c > 2.0 {
		t.Errorf("GetLoudnessCoefficient = %v, 预期约 1.995", c)
	}
	// 分析失败 (缓存 nil) 的歌曲不调整音量
	stubAnalyze(t, func(source, id string) {
		SetCachedLoudness(source, id, nil)
	})
	if c := GetLoudnessCoefficient("coef", "miss"); c != 1 {
		t.Errorf("分析失败时系数 = %v, 预期 1", c)
	}
}

func TestLoudnessCacheEviction(t *testing.T) {
	// 写入超过 1024 条, 最早写入的条目应被淘汰
	for i := 0; i < 1100; i++ {
		SetCachedLoudness("evict", fmt.Sprint(i), &loudness.Result{Integrated: -14})
	}
	if _, ok := GetCachedLoudness("evict", "0"); ok {
		t.Error("最早写入的条目应被 LRU 淘汰")
	}
	if _, ok := GetCachedLoudness("evict", "1099"); !ok {
		t.Error("最新写入的条目应保留")
	}
}

// stubAnalyze 注入假分析实现,返回调用计数。
func stubAnalyze(t *testing.T, fn func(source, id string)) *atomic.Int32 {
	t.Helper()
	orig := analyzeFunc
	var calls atomic.Int32
	analyzeFunc = func(source, id string) {
		calls.Add(1)
		fn(source, id)
	}
	t.Cleanup(func() { analyzeFunc = orig })
	return &calls
}

func TestGetLoudnessCoefficientSyncAnalyze(t *testing.T) {
	calls := stubAnalyze(t, func(source, id string) {
		SetCachedLoudness(source, id, &loudness.Result{Integrated: -20, Peak: -10})
	})

	// 未缓存时同步触发分析,并返回分析后的系数
	if c := GetLoudnessCoefficient("coef", "sync"); c < 1.99 || c > 2.0 {
		t.Fatalf("系数 = %v, 预期约 1.995", c)
	}
	if calls.Load() != 1 {
		t.Fatalf("分析调用次数 = %d, 预期 1", calls.Load())
	}
}

func TestGetLoudnessCoefficientCached(t *testing.T) {
	SetCachedLoudness("coef", "hit", &loudness.Result{Integrated: -14})
	calls := stubAnalyze(t, func(source, id string) {
		t.Error("已缓存的歌曲不应再分析")
	})

	done := make(chan struct{})
	go func() {
		GetLoudnessCoefficient("coef", "hit")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("已缓存时应立即返回")
	}
	_ = calls
}

func TestGetLoudnessCoefficientWaitsInflight(t *testing.T) {
	gate := make(chan struct{})
	calls := stubAnalyze(t, func(source, id string) {
		<-gate // 模拟慢速分析
		SetCachedLoudness(source, id, &loudness.Result{Integrated: -20, Peak: -10})
	})

	AnalyzeLoudnessAsync("coef", "wait")

	// 等异步任务登记并进入分析
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("异步分析未开始")
		}
		time.Sleep(time.Millisecond)
	}

	// 并发取系数应阻塞等待进行中的分析,而不是重复发起
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			GetLoudnessCoefficient("coef", "wait")
		}()
	}

	// 确认确实在等待 (未提前返回)
	time.Sleep(50 * time.Millisecond)
	close(gate)
	wg.Wait()

	if calls.Load() != 1 {
		t.Fatalf("分析调用次数 = %d, 预期 1 (不应重复分析)", calls.Load())
	}
	if c := GetLoudnessCoefficient("coef", "wait"); c < 1.99 || c > 2.0 {
		t.Fatalf("等待完成后系数 = %v, 预期约 1.995", c)
	}
}

func TestGetLoudnessCoefficientFailure(t *testing.T) {
	calls := stubAnalyze(t, func(source, id string) {
		SetCachedLoudness(source, id, nil) // 模拟失败: 缓存 nil 结果
	})

	if c := GetLoudnessCoefficient("coef", "fail"); c != 1 {
		t.Fatalf("分析失败后系数 = %v, 预期 1", c)
	}
	// 失败结果被缓存, 不应反复重试
	GetLoudnessCoefficient("coef", "fail")
	if calls.Load() != 1 {
		t.Fatalf("分析调用次数 = %d, 预期 1 (失败不应重试)", calls.Load())
	}
}
