//go:build !amd64

package loudness

// 非 amd64 平台没有 AVX2 内核,始终走标量路径。
const hasAVX2 = false

// stereoProcessAVX2 不会被调用 (hasAVX2 为 false)。
func stereoProcessAVX2(samples []float64, st *[2][2]biquadState, sums *[2]float64, peak *float64) {
	panic("loudness: AVX2 内核仅在 amd64 可用")
}
