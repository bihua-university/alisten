package wav

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	mp3 "github.com/hajimehoshi/go-mp3"
	"github.com/korandiz/mpa"
)

// newMP3Decoder 创建 MP3 流式解码器:跳过 ID3v2 标签后按首帧帧头选择解码后端。
// MPEG-1 Layer III 走 mpa 快路径,其余帧 (如 MPEG-2) 交给 go-mp3,
// MPEG-2.5 仍由 go-mp3 报不支持。两个后端都输出双声道交错采样。
func newMP3Decoder(r io.Reader) (Decoder, error) {
	br := bufio.NewReader(r)
	if err := skipID3v2(br); err != nil {
		return nil, fmt.Errorf("wav: 读取 MP3 标签失败: %w", err)
	}
	head, err := br.Peek(4)
	if err != nil {
		return nil, fmt.Errorf("wav: 读取 MP3 帧头失败: %w", err)
	}
	if isMPEG1LayerIII(head) {
		return newMPADecoder(br)
	}
	return newGoMP3Decoder(br)
}

// isMPEG1LayerIII 判断帧头前 4 字节是否为 mpa 可解码的
// MPEG-1 Layer III 帧 (码率与采样率索引不能是保留值)。
func isMPEG1LayerIII(h []byte) bool {
	return h[0] == 0xff && // 同步字高 8 位
		h[1]&0xfe == 0xfa && // 同步字低 3 位 + MPEG-1 + Layer III (忽略保护位)
		h[2]>>4 != 0x0f && h[2]&0x0c != 0x0c // 保留的码率/采样率索引
}

// skipID3v2 跳过文件开头连续的 ID3v2 标签,避免把封面等大块元数据读入内存。
func skipID3v2(r *bufio.Reader) error {
	for {
		head, err := r.Peek(3)
		if err != nil {
			return err
		}
		if string(head) != "ID3" {
			return nil
		}
		head, err = r.Peek(10)
		if err != nil {
			return err
		}
		// 标签长度是 4 字节 syncsafe 整数,每个字节最高位必须为 0。
		if head[6]|head[7]|head[8]|head[9] >= 0x80 {
			return errors.New("非法 ID3v2 标签长度")
		}
		size := int64(head[6])<<21 | int64(head[7])<<14 | int64(head[8])<<7 | int64(head[9])
		size += 10 // 标签头
		if head[3] == 4 && head[5]&0x10 != 0 {
			size += 10 // ID3v2.4 footer
		}
		if _, err := io.CopyN(io.Discard, r, size); err != nil {
			return err
		}
	}
}

// newMPADecoder 用 mpa 解码 MPEG-1 Layer III,输出双声道交错的浮点采样。
func newMPADecoder(r io.Reader) (Decoder, error) {
	dec := &mpa.Decoder{Input: r}
	d := &streamDecoder{ch: 2}
	// mpa 单帧每声道最多 1152 个采样,缓冲跨 fill 复用。
	var samples [1152]float32
	d.fill = func() error {
		if err := dec.DecodeFrame(); err != nil {
			return err
		}
		n := dec.NSamples()
		out := d.q.buf[d.q.grow(n*2):]
		for c := 0; c < 2; c++ {
			// 单声道时 c=1 也返回左声道,保证输出始终是双声道。
			dec.ReadSamples(c, samples[:n])
			for i, v := range samples[:n] {
				out[i*2+c] = float64(v)
			}
		}
		return nil
	}
	// 构造时先解出首帧:既借此确定采样率,也让采样留在队列里供首次 Read 消费。
	if err := d.fill(); err != nil {
		return nil, fmt.Errorf("wav: 初始化 MPEG-1 解码器失败: %w", err)
	}
	d.rate = dec.SamplingFrequency()
	return d, nil
}

// newGoMP3Decoder 用 go-mp3 解码,输出量化为 int16 的双声道采样。
func newGoMP3Decoder(r io.Reader) (Decoder, error) {
	dec, err := mp3.NewDecoder(r)
	if err != nil {
		return nil, fmt.Errorf("wav: 初始化 MP3 解码器失败: %w", err)
	}
	raw := make([]byte, 65536) // go-mp3 要求读取缓冲长度为 4 的倍数
	d := &streamDecoder{rate: dec.SampleRate(), ch: 2}
	d.fill = func() error {
		n, err := dec.Read(raw)
		out := d.q.buf[d.q.grow(n/2):]
		for i := range out {
			out[i] = float64(int16(binary.LittleEndian.Uint16(raw[i*2:]))) / 32768
		}
		if err == io.EOF && len(out) > 0 {
			return nil // 先交付尾部采样,下一次 fill 再返回 EOF
		}
		return err
	}
	return d, nil
}
