// Package wav 提供音频解码能力。
// NewDecoder 按 magic 自动识别 WAV / FLAC / MP3 并流式解码;
// 解码出的采样按声道交错,并归一化到 [-1, 1] 区间。
package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	mp3 "github.com/hajimehoshi/go-mp3"
	"github.com/mewkiz/flac"
)

// Audio 表示一段解码后的 PCM 音频。
type Audio struct {
	SampleRate int       // 采样率 (Hz)
	Channels   int       // 声道数
	Samples    []float64 // 按声道交错, 归一化到 [-1, 1]
}

// Frames 返回采样帧数 (每帧包含所有声道各一个采样)。
func (a *Audio) Frames() int {
	if a.Channels <= 0 {
		return 0
	}
	return len(a.Samples) / a.Channels
}

// Duration 返回音频时长。
func (a *Audio) Duration() time.Duration {
	if a.SampleRate <= 0 {
		return 0
	}
	return time.Duration(a.Frames()) * time.Second / time.Duration(a.SampleRate)
}

const (
	formatPCM        = 0x0001
	formatFloat      = 0x0003
	formatExtensible = 0xFFFE
)

// Decoder 是流式音频解码器,按块返回按声道交错、归一化到 [-1, 1] 的 PCM 采样。
type Decoder interface {
	// Read 解码并填充 p,返回采样个数;音频解码完毕时返回 io.EOF。
	Read(p []float64) (int, error)
	SampleRate() int
	Channels() int
}

// NewDecoder 按魔数自动识别格式 (WAV / FLAC / MP3) 并创建流式解码器。
func NewDecoder(r io.Reader) (Decoder, error) {
	var head [12]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, fmt.Errorf("wav: 读取文件头失败: %w", err)
	}
	r = io.MultiReader(bytes.NewReader(head[:]), r)
	switch {
	case string(head[0:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		return newWAVDecoder(r)
	case bytes.HasPrefix(head[:], []byte("fLaC")):
		return newFLACDecoder(r)
	case bytes.HasPrefix(head[:], []byte("ID3")) || (head[0] == 0xFF && head[1]&0xE0 == 0xE0):
		return newMP3Decoder(r)
	default:
		return nil, errors.New("wav: 无法识别的音频格式 (支持 WAV/FLAC/MP3)")
	}
}

// Decode 解码全部音频。大文件建议改用 NewDecoder 流式解码以节约内存。
func Decode(r io.Reader) (*Audio, error) {
	dec, err := NewDecoder(r)
	if err != nil {
		return nil, err
	}
	return drainDecoder(dec)
}

// drainDecoder 读取解码器的全部采样。
func drainDecoder(dec Decoder) (*Audio, error) {
	buf := make([]float64, 8192)
	samples := make([]float64, 0, 1<<20)
	for {
		n, err := dec.Read(buf)
		samples = append(samples, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if len(samples) == 0 {
		return nil, errors.New("wav: 音频流中没有采样数据")
	}
	return &Audio{SampleRate: dec.SampleRate(), Channels: dec.Channels(), Samples: samples}, nil
}

// sampleQueue 缓冲解码出的采样,供 Decoder.Read 分块取出。
type sampleQueue struct {
	buf []float64
	off int
}

func (q *sampleQueue) read(p []float64) int {
	n := copy(p, q.buf[q.off:])
	q.off += n
	if q.off >= len(q.buf) {
		q.buf = q.buf[:0]
		q.off = 0
	}
	return n
}

// grow 扩展队列以追加 n 个采样,返回追加区域的起始偏移。
// 容量足够时零分配复用底层数组。
func (q *sampleQueue) grow(n int) int {
	base := len(q.buf)
	if base+n > cap(q.buf) {
		nb := make([]float64, base+n)
		copy(nb, q.buf)
		q.buf = nb
	}
	q.buf = q.buf[:base+n]
	return base
}

// streamDecoder 是 Decoder 的通用实现:fill 负责向队列补充采样,Read 从队列取出。
type streamDecoder struct {
	q    sampleQueue
	rate int
	ch   int
	fill func() error
}

func (d *streamDecoder) Read(p []float64) (int, error) {
	for d.q.off >= len(d.q.buf) {
		if err := d.fill(); err != nil {
			return 0, err
		}
	}
	return d.q.read(p), nil
}

func (d *streamDecoder) SampleRate() int { return d.rate }
func (d *streamDecoder) Channels() int   { return d.ch }

// appendSamples 按 WAV 格式与位深将原始字节转换为归一化采样并追加到队列。
// 队列一次性按采样数扩容,避免逐样本 append 的扩容与 GC 开销。
func (d *streamDecoder) appendSamples(format, bits uint16, data []byte) {
	bps := int(bits) / 8
	if bps == 0 {
		return
	}
	n := len(data) / bps
	out := d.q.buf[d.q.grow(n):]
	switch format {
	case formatPCM:
		switch bits {
		case 8:
			for i, v := range data {
				out[i] = (float64(v) - 128) / 128
			}
		case 16:
			for i := range out {
				out[i] = float64(int16(binary.LittleEndian.Uint16(data[i*2:]))) / 32768
			}
		case 24:
			for i := range out {
				v := int32(data[i*3]) | int32(data[i*3+1])<<8 | int32(data[i*3+2])<<16
				if v&0x800000 != 0 {
					v |= ^0xFFFFFF // 符号扩展
				}
				out[i] = float64(v) / 8388608
			}
		case 32:
			for i := range out {
				out[i] = float64(int32(binary.LittleEndian.Uint32(data[i*4:]))) / 2147483648
			}
		}
	case formatFloat:
		switch bits {
		case 32:
			for i := range out {
				out[i] = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:])))
			}
		case 64:
			for i := range out {
				out[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[i*8:]))
			}
		}
	}
}

// wavHeader 是 RIFF/WAVE 头解析结果。
type wavHeader struct {
	format   uint16
	bits     uint16
	rate     int
	channels int
	dataSize int64
}

// newWAVDecoder 流式解析 RIFF/WAVE 头并创建解码器。
func newWAVDecoder(r io.Reader) (Decoder, error) {
	h, err := parseWAVHeader(r)
	if err != nil {
		return nil, err
	}
	bps := int(h.bits) / 8
	if bps == 0 {
		return nil, fmt.Errorf("wav: 不支持的位深 %d", h.bits)
	}
	if h.format != formatPCM && h.format != formatFloat {
		return nil, fmt.Errorf("wav: 不支持的音频格式 0x%x", h.format)
	}
	d := &streamDecoder{rate: h.rate, ch: h.channels}
	w := &wavDataReader{
		r:      r,
		format: h.format,
		bits:   h.bits,
		bps:    int64(bps),
		remain: h.dataSize,
		raw:    make([]byte, 65536),
	}
	d.fill = func() error { return w.fill(d) }
	return d, nil
}

// parseWAVHeader 顺序解析 RIFF/WAVE 头,返回时 r 正好定位在 data 块起始处。
func parseWAVHeader(r io.Reader) (wavHeader, error) {
	var hdr [12]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return wavHeader{}, errors.New("wav: 不是合法的 RIFF/WAVE 文件")
	}
	if string(hdr[0:4]) != "RIFF" || string(hdr[8:12]) != "WAVE" {
		return wavHeader{}, errors.New("wav: 不是合法的 RIFF/WAVE 文件")
	}

	var h wavHeader
	seenFmt := false
	for {
		var ch [8]byte
		if _, err := io.ReadFull(r, ch[:]); err != nil {
			return wavHeader{}, errors.New("wav: 缺少 data 块")
		}
		size := int64(binary.LittleEndian.Uint32(ch[4:]))
		id := string(ch[0:4])

		if id == "data" {
			if !seenFmt || h.channels == 0 || h.rate == 0 {
				return wavHeader{}, errors.New("wav: 缺少 fmt 块")
			}
			h.dataSize = size
			return h, nil
		}
		if id != "fmt " {
			// 跳过其他块 (含 2 字节对齐填充)
			if _, err := io.CopyN(io.Discard, r, size+size&1); err != nil {
				return wavHeader{}, errors.New("wav: 缺少 data 块")
			}
			continue
		}

		if size < 16 {
			return wavHeader{}, errors.New("wav: fmt 块过短")
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(r, buf); err != nil {
			return wavHeader{}, errors.New("wav: 读取 fmt 块失败")
		}
		h.format = binary.LittleEndian.Uint16(buf)
		h.channels = int(binary.LittleEndian.Uint16(buf[2:]))
		h.rate = int(binary.LittleEndian.Uint32(buf[4:]))
		h.bits = binary.LittleEndian.Uint16(buf[14:])
		if h.format == formatExtensible && size >= 40 {
			h.format = binary.LittleEndian.Uint16(buf[24:])
		}
		seenFmt = true
		if size&1 == 1 {
			io.CopyN(io.Discard, r, 1) // fmt 块的对齐填充
		}
	}
}

// wavDataReader 从 data 块中流式解码采样。
type wavDataReader struct {
	r      io.Reader
	format uint16
	bits   uint16
	bps    int64
	remain int64
	raw    []byte
}

// fill 解码下一块采样追加到队列,数据耗尽时返回 io.EOF。
func (w *wavDataReader) fill(d *streamDecoder) error {
	if w.remain <= 0 {
		return io.EOF
	}
	n := min(int64(len(w.raw)), w.remain)
	n -= n % w.bps // 按采样对齐
	if n == 0 {    // 尾部不足一个采样
		w.remain = 0
		return io.EOF
	}
	nn, err := io.ReadFull(w.r, w.raw[:n])
	w.remain -= int64(nn)
	if nn > 0 {
		d.appendSamples(w.format, w.bits, w.raw[:nn])
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}
	if err != nil && len(d.q.buf) == 0 {
		return err
	}
	return nil
}

// newMP3Decoder 创建 MP3 流式解码器 (go-mp3 统一输出双声道 16 bit 采样)。
func newMP3Decoder(r io.Reader) (Decoder, error) {
	dec, err := mp3.NewDecoder(r)
	if err != nil {
		return nil, fmt.Errorf("wav: 初始化 MP3 解码器失败: %w", err)
	}
	raw := make([]byte, 65536) // go-mp3 要求读取缓冲长度为 4 的倍数
	d := &streamDecoder{rate: dec.SampleRate(), ch: 2}
	d.fill = func() error {
		n, err := dec.Read(raw)
		// 队列一次性扩容,避免逐样本 append
		out := d.q.buf[d.q.grow(n/2):]
		for i := range out {
			out[i] = float64(int16(binary.LittleEndian.Uint16(raw[i*2:]))) / 32768
		}
		if err != nil {
			if err == io.EOF && len(d.q.buf) > 0 {
				return nil
			}
			return err
		}
		return nil
	}
	return d, nil
}

// newFLACDecoder 创建 FLAC 流式解码器,保持原始声道数与采样率。
func newFLACDecoder(r io.Reader) (Decoder, error) {
	stream, err := flac.New(r)
	if err != nil {
		return nil, fmt.Errorf("wav: 初始化 FLAC 解码器失败: %w", err)
	}
	info := stream.Info
	divisor := float64(int64(1) << (info.BitsPerSample - 1))
	d := &streamDecoder{rate: int(info.SampleRate), ch: int(info.NChannels)}
	d.fill = func() error {
		for {
			frame, err := stream.ParseNext()
			if err != nil {
				return err // 含 io.EOF
			}
			if len(frame.Subframes) == 0 {
				continue
			}
			n := frame.Subframes[0].NSamples
			ch := len(frame.Subframes)
			// 队列一次性扩容,按声道分列写入交错采样
			out := d.q.buf[d.q.grow(n*ch):]
			for c, sub := range frame.Subframes {
				for i := 0; i < n; i++ {
					out[i*ch+c] = float64(sub.Samples[i]) / divisor
				}
			}
			return nil
		}
	}
	return d, nil
}
