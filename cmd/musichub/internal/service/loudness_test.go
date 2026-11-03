package service

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bihua-university/alisten/cmd/musichub/internal/loudness"
)

func TestLoudnessCache(t *testing.T) {
	r := &loudness.Result{Integrated: -12.5, Peak: -1.2}
	loudnessCache.Add(loudnessKey("netease", "12345"), r)

	got, ok := loudnessCache.Get(loudnessKey("netease", "12345"))
	if !ok || got != r {
		t.Fatalf("缓存命中失败: ok=%v got=%v", ok, got)
	}

	if _, ok := loudnessCache.Get(loudnessKey("qq", "12345")); ok {
		t.Error("不同 source 不应命中同一缓存")
	}
	if _, ok := loudnessCache.Get(loudnessKey("netease", "99999")); ok {
		t.Error("不同 id 不应命中同一缓存")
	}
}

func TestLoudnessCacheEviction(t *testing.T) {
	// 写入超过 1024 条, 最早写入的条目应被淘汰
	for i := 0; i < 1100; i++ {
		loudnessCache.Add(loudnessKey("evict", fmt.Sprint(i)), &loudness.Result{Integrated: -14})
	}
	if _, ok := loudnessCache.Get(loudnessKey("evict", "0")); ok {
		t.Error("最早写入的条目应被 LRU 淘汰")
	}
	if _, ok := loudnessCache.Get(loudnessKey("evict", "1099")); !ok {
		t.Error("最新写入的条目应保留")
	}
}

// stubLoudnessCompute 注入假计算实现,返回调用计数。
func stubLoudnessCompute(t *testing.T, fn func(source, id string) (*loudness.Result, error)) *atomic.Int32 {
	t.Helper()
	orig := loudnessCompute
	var calls atomic.Int32
	loudnessCompute = func(source, id string) (*loudness.Result, error) {
		calls.Add(1)
		return fn(source, id)
	}
	t.Cleanup(func() { loudnessCompute = orig })
	return &calls
}

func TestGetLoudnessCoefficientCached(t *testing.T) {
	loudnessCache.Add(loudnessKey("coef", "hit"), &loudness.Result{Integrated: -20, Peak: -10})
	calls := stubLoudnessCompute(t, func(source, id string) (*loudness.Result, error) {
		t.Error("已缓存的结果不应再计算")
		return nil, nil
	})

	// -14 - (-20) = +6 dB -> 系数约 1.995
	if c := GetLoudnessCoefficient("coef", "hit"); c < 1.99 || c > 2.0 {
		t.Fatalf("系数 = %v, 预期约 1.995", c)
	}
	if calls.Load() != 0 {
		t.Fatalf("计算次数 = %d, 预期 0", calls.Load())
	}
}

func TestGetLoudnessCoefficientComputes(t *testing.T) {
	calls := stubLoudnessCompute(t, func(source, id string) (*loudness.Result, error) {
		return &loudness.Result{Integrated: -20, Peak: -10}, nil
	})

	// 未缓存时同步计算,并返回计算后的系数
	if c := GetLoudnessCoefficient("coef", "sync"); c < 1.99 || c > 2.0 {
		t.Fatalf("系数 = %v, 预期约 1.995", c)
	}
	if calls.Load() != 1 {
		t.Fatalf("计算次数 = %d, 预期 1", calls.Load())
	}
}

func TestGetLoudnessCoefficientFailure(t *testing.T) {
	calls := stubLoudnessCompute(t, func(source, id string) (*loudness.Result, error) {
		return nil, fmt.Errorf("下载失败")
	})

	if c := GetLoudnessCoefficient("coef", "fail"); c != 1 {
		t.Fatalf("计算失败后系数 = %v, 预期 1", c)
	}
	// 失败结果被缓存, 不应反复重试
	GetLoudnessCoefficient("coef", "fail")
	if calls.Load() != 1 {
		t.Fatalf("计算次数 = %d, 预期 1 (失败不应重试)", calls.Load())
	}
}

func TestGetLoudnessCoefficientSharesInflight(t *testing.T) {
	gate := make(chan struct{})
	started := make(chan struct{}, 4)
	calls := stubLoudnessCompute(t, func(source, id string) (*loudness.Result, error) {
		started <- struct{}{}
		<-gate // 模拟慢速下载/解码
		return &loudness.Result{Integrated: -20, Peak: -10}, nil
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		GetLoudnessCoefficient("coef", "wait")
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("响度计算未开始")
	}

	// 并发请求应等待同一次计算,而不是重复发起
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
		t.Fatalf("计算次数 = %d, 预期 1 (不应重复计算)", calls.Load())
	}
	if c := GetLoudnessCoefficient("coef", "wait"); c < 1.99 || c > 2.0 {
		t.Fatalf("等待完成后系数 = %v, 预期约 1.995", c)
	}
}
