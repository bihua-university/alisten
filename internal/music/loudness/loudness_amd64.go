//go:build amd64

package loudness

import "golang.org/x/sys/cpu"

// hasAVX2 报告 CPU 是否支持 AVX2,不支持时回退到标量路径。
var hasAVX2 = cpu.X86.HasAVX2

// stereoProcessAVX2 用 AVX2 (128-bit VEX) 处理双声道交错采样:
// 每帧并行更新 L/R 声道的峰值与两级 K 加权 biquad 状态,
// 滤波后能量累加进 sums。samples 长度必须为偶数。
// st 为两个声道各两级 biquadState (内存布局见 loudness_amd64.s)。
//
//go:noescape
func stereoProcessAVX2(samples []float64, st *[2][2]biquadState, sums *[2]float64, peak *float64)
