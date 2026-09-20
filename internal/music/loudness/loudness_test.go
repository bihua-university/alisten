package loudness

import (
	"math"
	"math/rand"
	"testing"
)

// sine 生成指定振幅的正弦波 (双声道, 交错采样)。
func sine(sampleRate int, freq, amp, seconds float64) []float64 {
	frames := int(float64(sampleRate) * seconds)
	samples := make([]float64, frames*2)
	for f := 0; f < frames; f++ {
		v := amp * math.Sin(2*math.Pi*freq*float64(f)/float64(sampleRate))
		samples[f*2] = v
		samples[f*2+1] = v
	}
	return samples
}

func TestAnalyzeSamplesSine(t *testing.T) {
	const fs = 48000
	// 997 Hz, 振幅 0.25: 原始 RMS 对应约 -12.73 LUFS,
	// K 加权在 1 kHz 附近约有 +0.6 dB 增益, 预期约 -12.1 LUFS。
	res, err := AnalyzeSamples(sine(fs, 997, 0.25, 3), fs, 2)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(res.Integrated-(-12.1)) > 1.0 {
		t.Errorf("Integrated = %.2f LUFS, 预期约 -12.1", res.Integrated)
	}
	if math.Abs(res.Peak-(-12.04)) > 0.5 {
		t.Errorf("Peak = %.2f dBFS, 预期约 -12.04", res.Peak)
	}
	if res.Duration < 2900*1e6 || res.Duration > 3100*1e6 {
		t.Errorf("Duration = %v, 预期约 3s", res.Duration)
	}
}

func TestAnalyzeSamplesRelative(t *testing.T) {
	const fs = 44100
	// 振幅相差一倍, 响度应相差约 6.02 LU (门控与滤波对两者一致)
	quiet, err := AnalyzeSamples(sine(fs, 997, 0.125, 3), fs, 2)
	if err != nil {
		t.Fatal(err)
	}
	loud, err := AnalyzeSamples(sine(fs, 997, 0.25, 3), fs, 2)
	if err != nil {
		t.Fatal(err)
	}
	if diff := loud.Integrated - quiet.Integrated; math.Abs(diff-6.02) > 0.2 {
		t.Errorf("响度差 = %.2f LU, 预期约 6.02", diff)
	}
}

func TestAnalyzeSamplesSilence(t *testing.T) {
	res, err := AnalyzeSamples(make([]float64, 48000*2), 48000, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !math.IsInf(res.Integrated, -1) {
		t.Errorf("静音 Integrated = %v, 预期 -Inf", res.Integrated)
	}
	if g := res.SpotifyGain(); g != 0 {
		t.Errorf("静音 SpotifyGain = %v, 预期 0", g)
	}
}

func TestAnalyzeSamplesInvalid(t *testing.T) {
	if _, err := AnalyzeSamples(nil, 48000, 2); err == nil {
		t.Error("空采样应返回错误")
	}
	if _, err := AnalyzeSamples([]float64{0}, 0, 2); err == nil {
		t.Error("非法采样率应返回错误")
	}
}

func TestSpotifyGain(t *testing.T) {
	cases := []struct {
		name string
		r    Result
		want float64
	}{
		// 响度 -20 LUFS, 峰值 -10 dBFS: 增益 +6, 峰值到 -4, 不触发限制
		{"正常提升", Result{Integrated: -20, Peak: -10}, 6},
		// 响度 -20 LUFS, 峰值 -2 dBFS: 增益 +6 会使峰值到 +4, 被限制到 -1-(-2)=+1
		{"峰值限制", Result{Integrated: -20, Peak: -2}, 1},
		// 响度 -10 LUFS 高于目标: 增益 -4
		{"需要降低", Result{Integrated: -10, Peak: -6}, -4},
		// 恰好达到目标
		{"无需调整", Result{Integrated: -14, Peak: -6}, 0},
	}
	for _, c := range cases {
		if got := c.r.SpotifyGain(); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: SpotifyGain = %v, 预期 %v", c.name, got, c.want)
		}
	}
}

func TestGainTo(t *testing.T) {
	r := &Result{Integrated: -20}
	if g := r.GainTo(-23); g != -3 {
		t.Errorf("GainTo(-23) = %v, 预期 -3", g)
	}
}

func TestCoefficient(t *testing.T) {
	// -14 - (-20) = +6 dB -> 系数约 1.995
	r := &Result{Integrated: -20, Peak: -10}
	if c := r.Coefficient(); math.Abs(c-1.9953) > 0.001 {
		t.Errorf("Coefficient = %v, 预期约 1.995", c)
	}
	// 响度高于目标 -8 LUFS: -14 - (-8) = -6 dB -> 系数约 0.501
	r = &Result{Integrated: -8, Peak: -3}
	if c := r.Coefficient(); math.Abs(c-0.5012) > 0.001 {
		t.Errorf("Coefficient = %v, 预期约 0.501", c)
	}
	// 静音不调整
	r = &Result{Integrated: math.Inf(-1), Peak: math.Inf(-1)}
	if c := r.Coefficient(); c != 1 {
		t.Errorf("静音 Coefficient = %v, 预期 1", c)
	}
}

func TestAnalyzerChunkedParity(t *testing.T) {
	const fs = 44100
	samples := sine(fs, 997, 0.25, 3)

	whole, err := AnalyzeSamples(samples, fs, 2)
	if err != nil {
		t.Fatal(err)
	}

	// 以不规整的小块喂入 (奇数长度, 跨声道/跨段边界)
	an := NewAnalyzer(fs, 2)
	for off := 0; off < len(samples); {
		n := min(997, len(samples)-off)
		an.Write(samples[off : off+n])
		off += n
	}
	chunked, err := an.Result()
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(chunked.Integrated-whole.Integrated) > 1e-6 {
		t.Errorf("流式 Integrated = %v, 整体 = %v", chunked.Integrated, whole.Integrated)
	}
	if chunked.Peak != whole.Peak {
		t.Errorf("流式 Peak = %v, 整体 = %v", chunked.Peak, whole.Peak)
	}
	if chunked.Duration != whole.Duration {
		t.Errorf("流式 Duration = %v, 整体 = %v", chunked.Duration, whole.Duration)
	}
}

// TestAVX2ScalarParity 对比 AVX2 内核与纯 Go 标量路径的分析结果。
func TestAVX2ScalarParity(t *testing.T) {
	if !hasAVX2 {
		t.Skip("CPU 不支持 AVX2")
	}
	const fs = 48000
	// 伪随机采样 + 静音段, 覆盖门控与滤波状态
	rng := rand.New(rand.NewSource(42))
	samples := make([]float64, fs*5*2)
	for i := range samples {
		if i/(fs/5)%2 == 0 {
			samples[i] = rng.Float64()*2 - 1
		}
	}

	run := func(simd bool) *Result {
		an := NewAnalyzer(fs, 2)
		an.simdOK = simd
		for off := 0; off < len(samples); {
			n := min(999, len(samples)-off) // 奇数块, 触发标量边界处理
			an.Write(samples[off : off+n])
			off += n
		}
		res, err := an.Result()
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	want := run(false)
	got := run(true)
	if diff := math.Abs(got.Integrated - want.Integrated); diff > 1e-6 {
		t.Errorf("Integrated 偏差 = %v (AVX2 %v vs 标量 %v)", diff, got.Integrated, want.Integrated)
	}
	if got.Peak != want.Peak {
		t.Errorf("Peak: AVX2 %v vs 标量 %v", got.Peak, want.Peak)
	}
	if got.Duration != want.Duration {
		t.Errorf("Duration: AVX2 %v vs 标量 %v", got.Duration, want.Duration)
	}
}
