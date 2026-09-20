package loudness

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// benchSamples 生成 60 秒 48kHz 双声道正弦波,模拟真实长度的音频分析负载。
func benchSamples() []float64 {
	const fs = 48000
	frames := fs * 60
	samples := make([]float64, frames*2)
	for f := 0; f < frames; f++ {
		v := 0.25 * math.Sin(2*math.Pi*997*float64(f)/fs)
		samples[f*2] = v
		samples[f*2+1] = v
	}
	return samples
}

func BenchmarkAnalyzeSamples(b *testing.B) {
	samples := benchSamples()
	b.SetBytes(int64(len(samples)) * 8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := AnalyzeSamples(samples, 48000, 2); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAnalyzerWrite(b *testing.B) {
	samples := benchSamples()
	b.SetBytes(int64(len(samples)) * 8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		an := NewAnalyzer(48000, 2)
		an.Write(samples)
		if _, err := an.Result(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAnalyzerWriteScalar 强制关闭 SIMD, 作为 AVX2 内核的对照组。
func BenchmarkAnalyzerWriteScalar(b *testing.B) {
	samples := benchSamples()
	b.SetBytes(int64(len(samples)) * 8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		an := NewAnalyzer(48000, 2)
		an.simdOK = false
		an.Write(samples)
		if _, err := an.Result(); err != nil {
			b.Fatal(err)
		}
	}
}

// benchWAV 生成 3 分钟 44.1kHz 16-bit 立体声 WAV (约 31MB)。
func benchWAV() []byte {
	const (
		fs      = 44100
		seconds = 180
		ch      = 2
		bits    = 16
	)
	frames := fs * seconds
	dataSize := frames * ch * bits / 8
	buf := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(buf, binary.LittleEndian, uint16(ch))
	binary.Write(buf, binary.LittleEndian, uint32(fs))
	binary.Write(buf, binary.LittleEndian, uint32(fs*ch*bits/8))
	binary.Write(buf, binary.LittleEndian, uint16(ch*bits/8))
	binary.Write(buf, binary.LittleEndian, uint16(bits))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	for f := 0; f < frames; f++ {
		v := int16(0.25 * 32767 * math.Sin(2*math.Pi*997*float64(f)/fs))
		binary.Write(buf, binary.LittleEndian, v)
		binary.Write(buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

// BenchmarkAnalyzeReaderWAV 测量 3 分钟歌曲 "解码 + 响度分析" 的端到端耗时。
func BenchmarkAnalyzeReaderWAV(b *testing.B) {
	data := benchWAV()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := AnalyzeReader(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}
