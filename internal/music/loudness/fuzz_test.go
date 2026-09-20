package loudness

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func fuzzSamples(data []byte, maxSamples int) []float64 {
	n := len(data) / 8
	if n > maxSamples {
		n = maxSamples
	}
	samples := make([]float64, n)
	for i := range samples {
		v := math.Float64frombits(binary.LittleEndian.Uint64(data[i*8:]))
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = 0
		}
		samples[i] = math.Max(-8, math.Min(8, v))
	}
	return samples
}

func FuzzAnalyzerSIMDParity(f *testing.F) {
	if !hasAVX2 {
		f.Skip("CPU 不支持 AVX2")
	}
	f.Add([]byte{}, uint16(0))
	f.Add(make([]byte, 512), uint16(7))
	f.Add(bytes.Repeat([]byte{0xFF}, 1024), uint16(513))

	f.Fuzz(func(t *testing.T, data []byte, split uint16) {
		samples := fuzzSamples(data, 16384)
		sp := int(split) % (len(samples) + 1)

		run := func(simd bool) (*Result, error) {
			an := NewAnalyzer(48000, 2)
			an.simdOK = simd
			an.Write(samples[:sp])
			an.Write(samples[sp:])
			return an.Result()
		}

		want, errWant := run(false)
		got, errGot := run(true)
		if (errWant == nil) != (errGot == nil) {
			t.Fatalf("错误不一致: 标量 %v, SIMD %v", errWant, errGot)
		}
		if errWant != nil {
			return
		}
		if math.IsInf(want.Integrated, -1) {
			if !math.IsInf(got.Integrated, -1) {
				t.Fatalf("Integrated: 标量 -Inf, SIMD %v", got.Integrated)
			}
		} else if math.Abs(got.Integrated-want.Integrated) > 1e-9 {
			t.Fatalf("Integrated 偏差过大: 标量 %v, SIMD %v", want.Integrated, got.Integrated)
		}
		if got.Peak != want.Peak {
			t.Fatalf("Peak 不一致: 标量 %v, SIMD %v", want.Peak, got.Peak)
		}
		if got.Duration != want.Duration {
			t.Fatalf("Duration 不一致: 标量 %v, SIMD %v", want.Duration, got.Duration)
		}
	})
}
