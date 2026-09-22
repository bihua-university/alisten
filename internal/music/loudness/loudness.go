// Package loudness 实现 ITU-R BS.1770 (EBU R128) 响度分析,
// 并按 Spotify 播放器的响度归一化策略给出平衡增益 (ReplayGain 风格)。
//
// Spotify 的策略要点:
//   - 目标响度为 -14 LUFS (Normal 档);
//   - 增益 = 目标响度 - 实测响度;
//   - 正向增益会被限制,保证采样峰值不超过 -1 dBFS,避免削波。
package loudness

import (
	"errors"
	"io"
	"math"
	"time"

	"github.com/bihua-university/alisten/internal/music/wav"
)

// SpotifyTarget 是 Spotify 播放器响度归一化的目标响度 (LUFS)。
const SpotifyTarget = -14.0

// peakCeiling 是 Spotify 提升音量时允许的采样峰值上限 (dBFS)。
const peakCeiling = -1.0

// Result 是一次响度分析的结果。
type Result struct {
	Integrated float64       // 综合响度 (LUFS), 全静音时为 -Inf
	Peak       float64       // 采样峰值 (dBFS), 全静音时为 -Inf
	Duration   time.Duration // 音频时长
}

// GainTo 返回将响度归一化到 target (LUFS) 所需的增益 (dB);静音返回 0。
func (r *Result) GainTo(target float64) float64 {
	if math.IsInf(r.Integrated, -1) {
		return 0
	}
	return target - r.Integrated
}

// SpotifyGain 按 Spotify 播放器策略返回平衡增益 (dB):
// 以 -14 LUFS 为目标;当需要提升音量时,限制增益使采样峰值不超过 -1 dBFS。
func (r *Result) SpotifyGain() float64 {
	g := r.GainTo(SpotifyTarget)
	if g > 0 && !math.IsInf(r.Peak, -1) && r.Peak+g > peakCeiling {
		g = peakCeiling - r.Peak
	}
	return g
}

// Coefficient 返回线性音量系数,静音返回 1 (不调整)。
func (r *Result) Coefficient() float64 {
	return GainToCoefficient(r.SpotifyGain())
}

// GainToCoefficient 将增益 (dB) 转换为线性音量系数。
func GainToCoefficient(gainDb float64) float64 {
	return math.Pow(10, gainDb/20)
}

// Analyze 对已解码的音频做响度分析。
func Analyze(a *wav.Audio) (*Result, error) {
	return AnalyzeSamples(a.Samples, a.SampleRate, a.Channels)
}

// AnalyzeReader 从音频流中流式解码 (自动识别 WAV/FLAC/MP3) 并分析响度。
func AnalyzeReader(r io.Reader) (*Result, error) {
	dec, err := wav.NewDecoder(r)
	if err != nil {
		return nil, err
	}
	an := NewAnalyzer(dec.SampleRate(), dec.Channels())
	buf := make([]float64, 32768)
	for {
		n, err := dec.Read(buf)
		an.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return an.Result()
}

// AnalyzeSamples 对交错的 PCM 采样 (归一化到 [-1, 1]) 做 BS.1770 综合响度分析。
func AnalyzeSamples(samples []float64, sampleRate, channels int) (*Result, error) {
	if sampleRate <= 0 || channels <= 0 {
		return nil, errors.New("loudness: 采样率与声道数必须为正")
	}
	an := NewAnalyzer(sampleRate, channels)
	an.Write(samples[:len(samples)/channels*channels])
	return an.Result()
}

// blockSegs 是一个 400ms 响度块包含的 100ms 段数 (BS.1770: 400ms 块, 75% 重叠)。
const blockSegs = 4

// Analyzer 是流式 BS.1770 响度分析器: 边解码边喂入采样,
// 只保留滤波器状态与滚动的块能量, 整段音频无需驻留内存。
type Analyzer struct {
	sampleRate int
	channels   int
	segSize    int                  // 100ms 段的帧数
	filters    [][2]biquadState     // 每声道两级 K 加权滤波器状态
	segSums    []float64            // 当前段各声道能量
	totalSums  []float64            // 全部采样的各声道能量 (不足 400ms 时兜底)
	window     [blockSegs][]float64 // 最近 4 段的各声道能量
	wpos, wlen int                  // 环形窗口写入位置与已填充段数
	segFrames  int                  // 当前段已积累的帧数
	nextCh     int                  // 下一个采样所属声道 (跨 Write 调用保持帧对齐)
	energies   []float64            // 每个完整 400ms 块的能量
	peak       float64              // 采样峰值 (未经过 K 加权滤波)
	frames     int64

	simdOK bool // 双声道且 CPU 支持 AVX2 时启用 SIMD 快路径
}

// NewAnalyzer 创建流式响度分析器。
func NewAnalyzer(sampleRate, channels int) *Analyzer {
	segSize := int(0.100 * float64(sampleRate))
	if segSize < 1 {
		segSize = 1
	}
	a := &Analyzer{
		sampleRate: sampleRate,
		channels:   channels,
		segSize:    segSize,
		filters:    make([][2]biquadState, channels),
		segSums:    make([]float64, channels),
		totalSums:  make([]float64, channels),
		simdOK:     channels == 2 && hasAVX2,
	}
	for c := 0; c < channels; c++ {
		shelf, highpass := kWeightingFilters(sampleRate)
		a.filters[c] = [2]biquadState{{b: shelf}, {b: highpass}}
	}
	for i := range a.window {
		a.window[i] = make([]float64, channels)
	}
	return a
}

// Write 喂入一块交错的 PCM 采样,块长度无需按声道或段边界对齐。
func (a *Analyzer) Write(samples []float64) {
	if a.simdOK {
		a.writeStereoSIMD(samples)
		return
	}
	for _, v := range samples {
		a.stepScalar(v)
	}
}

// writeStereoSIMD 是双声道 AVX2 快路径: 逐段调用汇编内核,
// 并行处理 L/R 声道的峰值、两级 biquad 与能量累加 (IIR 时间维串行, 声道维并行)。
func (a *Analyzer) writeStereoSIMD(samples []float64) {
	for len(samples) > 0 {
		if a.nextCh == 1 || len(samples) < 2 {
			// 奇数边界: 标量处理一个采样以保持帧对齐
			a.stepScalar(samples[0])
			samples = samples[1:]
			continue
		}
		n := a.segSize - a.segFrames // 当前段剩余帧数
		if rem := len(samples) / 2; rem < n {
			n = rem
		}
		before0, before1 := a.segSums[0], a.segSums[1]
		stereoProcessAVX2(samples[:2*n], (*[2][2]biquadState)(a.filters), (*[2]float64)(a.segSums), &a.peak)
		a.totalSums[0] += a.segSums[0] - before0
		a.totalSums[1] += a.segSums[1] - before1
		a.frames += int64(n)
		a.segFrames += n
		samples = samples[2*n:]
		if a.segFrames == a.segSize {
			a.flushSegment()
		}
	}
}

// stepScalar 标量处理一个采样: 峰值检测、两级 K 加权滤波、能量累加与段推进。
func (a *Analyzer) stepScalar(v float64) {
	if av := math.Abs(v); av > a.peak {
		a.peak = av
	}
	c := a.nextCh
	f := &a.filters[c]
	y := f[1].step(f[0].step(v))
	e := y * y
	a.segSums[c] += e
	a.totalSums[c] += e
	if c++; c == a.channels {
		c = 0
		a.frames++
		if a.segFrames++; a.segFrames == a.segSize {
			a.flushSegment()
		}
	}
	a.nextCh = c
}

// flushSegment 完成一个 100ms 段:推入环形窗口,窗口满 4 段时产出一个 400ms 块能量。
// 块能量为各声道均方值之和 (声道权重均为 1)。
func (a *Analyzer) flushSegment() {
	w := a.window[a.wpos]
	copy(w, a.segSums)
	a.wpos = (a.wpos + 1) % blockSegs
	if a.wlen < blockSegs {
		a.wlen++
	}
	clear(a.segSums)
	a.segFrames = 0

	if a.wlen == blockSegs {
		e := 0.0
		for i := 0; i < blockSegs; i++ {
			for _, s := range a.window[i] {
				e += s
			}
		}
		a.energies = append(a.energies, e/float64(blockSegs*a.segSize))
	}
}

// Result 结束分析并返回结果。没有任何采样时返回错误。
func (a *Analyzer) Result() (*Result, error) {
	if a.frames == 0 {
		return nil, errors.New("loudness: 没有音频采样")
	}

	energies := a.energies
	if len(energies) == 0 {
		// 音频不足 400ms, 用整段能量
		e := 0.0
		for _, s := range a.totalSums {
			e += s
		}
		energies = []float64{e / float64(a.frames)}
	}

	integrated := math.Inf(-1)
	if e, ok := gateBlocks(energies); ok {
		integrated = energyToLoudness(e)
	}

	peakDb := math.Inf(-1)
	if a.peak > 0 {
		peakDb = 20 * math.Log10(a.peak)
	}

	return &Result{
		Integrated: integrated,
		Peak:       peakDb,
		Duration:   time.Duration(a.frames) * time.Second / time.Duration(a.sampleRate),
	}, nil
}

// energyToLoudness 将块能量转换为响度 (LUFS),包含 BS.1770 的 -0.691 修正项。
func energyToLoudness(e float64) float64 {
	return -0.691 + 10*math.Log10(e)
}

// gateBlocks 执行 BS.1770 门控:先以 -70 LUFS 绝对门限过滤,
// 再以剩余块均值以下 10 LU 的相对门限过滤,返回最终参与综合响度计算的平均能量。
func gateBlocks(energies []float64) (float64, bool) {
	// BS.1770 的 -70 LUFS 绝对门限,直接换算到能量域比较,省去逐块的 LUFS 对数换算。
	absGate := math.Pow(10, (-70+0.691)/10)
	sum, n := sumAbove(energies, absGate)
	if n == 0 {
		return 0, false
	}
	// 相对门限是均值响度以下 10 LU,即均值能量的 1/10;与绝对门限取大,保证两级门限同时生效。
	sum, n = sumAbove(energies, max(absGate, sum/float64(n)/10))
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// sumAbove 返回 energies 中严格大于 threshold 的元素之和与个数。
func sumAbove(energies []float64, threshold float64) (float64, int) {
	sum, n := 0.0, 0
	for _, e := range energies {
		if e > threshold {
			sum += e
			n++
		}
	}
	return sum, n
}

// biquad 是直接 I 型二阶 IIR 滤波器系数。
type biquad struct {
	b0, b1, b2, a1, a2 float64
}

// biquadState 是带历史状态的 biquad,可跨块连续处理采样。
type biquadState struct {
	b              biquad
	x1, x2, y1, y2 float64
}

func (s *biquadState) step(v float64) float64 {
	y := s.b.b0*v + s.b.b1*s.x1 + s.b.b2*s.x2 - s.b.a1*s.y1 - s.b.a2*s.y2
	s.x2, s.x1 = s.x1, v
	s.y2, s.y1 = s.y1, y
	return y
}

// kWeightingFilters 按采样率计算 BS.1770 K 加权的两级滤波器系数
// (一级高频搁架 + 二级高通,系数取自 De Man 的 BS.1770 系数推导)。
func kWeightingFilters(sampleRate int) (shelf, highpass biquad) {
	fs := float64(sampleRate)

	// 一级: 高频搁架 (pre-filter)
	{
		const (
			f0 = 1681.974450955533
			G  = 3.999843853973347
			Q  = 0.7071752369554196
		)
		K := math.Tan(math.Pi * f0 / fs)
		Vh := math.Pow(10, G/20)
		Vb := math.Pow(Vh, 0.499666774155)
		a0 := 1 + K/Q + K*K
		shelf = biquad{
			b0: (Vh + Vb*K/Q + K*K) / a0,
			b1: 2 * (K*K - Vh) / a0,
			b2: (Vh - Vb*K/Q + K*K) / a0,
			a1: 2 * (K*K - 1) / a0,
			a2: (1 - K/Q + K*K) / a0,
		}
	}

	// 二级: 高通 (RLB)
	{
		const (
			f0 = 38.13547087602444
			Q  = 0.5003270373238773
		)
		K := math.Tan(math.Pi * f0 / fs)
		a0 := 1 + K/Q + K*K
		highpass = biquad{
			b0: 1 / a0,
			b1: -2 / a0,
			b2: 1 / a0,
			a1: 2 * (K*K - 1) / a0,
			a2: (1 - K/Q + K*K) / a0,
		}
	}
	return shelf, highpass
}
