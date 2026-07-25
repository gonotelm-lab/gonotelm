package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func makePCM(freq int, samples int, channels uint16, sampleRate uint32, bitsPerSample uint16) []byte {
	bytesPerSample := int(bitsPerSample) / 8
	frameCount := samples
	pcm := make([]byte, frameCount*int(channels)*bytesPerSample)
	for i := 0; i < frameCount; i++ {
		// 用简单 sawtooth 保证非零、可校验
		val := uint16(i % 1024)
		for ch := 0; ch < int(channels); ch++ {
			off := (i*int(channels) + ch) * bytesPerSample
			switch bitsPerSample {
			case 16:
				binary.LittleEndian.PutUint16(pcm[off:off+2], val)
			case 8:
				pcm[off] = byte(val & 0xFF)
			}
		}
	}
	_ = freq
	return pcm
}

func TestEncodeParseRoundTrip(t *testing.T) {
	original := makePCM(440, 100, 1, 16000, 16)
	wavBytes := Encode(original, 1, 16000, 16)

	parsed, err := Parse(wavBytes)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if parsed.NumChannels != 1 ||
		parsed.SampleRate != 16000 ||
		parsed.BitsPerSample != 16 {
		t.Fatalf("format mismatch: %+v", parsed)
	}
	if !bytes.Equal(parsed.Data, original) {
		t.Fatalf("pcm mismatch: len orig=%d parsed=%d", len(original), len(parsed.Data))
	}
}

func TestParse_NonPCM_Rejected(t *testing.T) {
	// 构造一个 format=3 (IEEE float) 的 wav
	pcm := make([]byte, 8)
	wav := Encode(pcm, 1, 16000, 32)
	// 改 format 字段为 3
	binary.LittleEndian.PutUint16(wav[20:22], 3)

	_, err := Parse(wav)
	if err == nil {
		t.Fatal("expected error for non-PCM format")
	}
}

func TestParse_TruncatedWAV(t *testing.T) {
	cases := [][]byte{
		nil,
		[]byte("RIFF"),
		[]byte("RIFF\x00\x00\x00\x00WAVE"), // missing chunks
		[]byte("NOTRIFF"),
	}
	for _, c := range cases {
		_, err := Parse(c)
		if err == nil {
			t.Fatalf("expected error for input len=%d", len(c))
		}
	}
}

func TestConcatPCMs_SameFormat(t *testing.T) {
	a := makePCM(1, 50, 1, 16000, 16)
	b := makePCM(1, 50, 1, 16000, 16)
	parts := []*PCM{
		{Data: a, NumChannels: 1, SampleRate: 16000, BitsPerSample: 16},
		{Data: b, NumChannels: 1, SampleRate: 16000, BitsPerSample: 16},
	}
	wav, merged, err := ConcatPCMs(parts)
	if err != nil {
		t.Fatalf("concat failed: %v", err)
	}
	if merged == nil {
		t.Fatal("merged nil")
	}
	if len(merged.Data) != len(a)+len(b) {
		t.Fatalf("merged len mismatch: got=%d want=%d", len(merged.Data), len(a)+len(b))
	}
	// 校验 encode 出来 parse 回去能拿到合并后的 PCM
	parsed, err := Parse(wav)
	if err != nil {
		t.Fatalf("parse concat wav failed: %v", err)
	}
	if !bytes.Equal(parsed.Data, merged.Data) {
		t.Fatal("concat wav data mismatch after parse")
	}
}

func TestConcatPCMs_IncompatibleFormat(t *testing.T) {
	parts := []*PCM{
		{Data: []byte{1, 2}, NumChannels: 1, SampleRate: 16000, BitsPerSample: 16},
		{Data: []byte{3, 4}, NumChannels: 2, SampleRate: 16000, BitsPerSample: 16},
	}
	_, _, err := ConcatPCMs(parts)
	if err == nil {
		t.Fatal("expected format mismatch error")
	}
}

func TestConcat_NilParts(t *testing.T) {
	_, _, err := ConcatPCMs(nil)
	if err == nil {
		t.Fatal("expected error for empty parts")
	}
	// nil part 需要报错
	parts := []*PCM{
		{Data: []byte{1, 2}, NumChannels: 1, SampleRate: 16000, BitsPerSample: 16},
		nil,
	}
	_, _, err = ConcatPCMs(parts)
	if err == nil {
		t.Fatal("expected error for nil entry in parts")
	}
}

func TestConcat_FromWAVBytes(t *testing.T) {
	a := Encode(makePCM(1, 30, 1, 22050, 16), 1, 22050, 16)
	b := Encode(makePCM(1, 30, 1, 22050, 16), 1, 22050, 16)
	merged, err := Concat([][]byte{a, b})
	if err != nil {
		t.Fatalf("concat from bytes failed: %v", err)
	}
	parsed, err := Parse(merged)
	if err != nil {
		t.Fatalf("parse merged failed: %v", err)
	}
	if parsed.SampleRate != 22050 {
		t.Fatalf("sample rate mismatch: got=%d want=22050", parsed.SampleRate)
	}
}

// 确保 Parse 在有 LIST/fact 等附加 chunk 时仍能正确取到 data。
func TestParse_SkipsExtraChunks(t *testing.T) {
	pcm := makePCM(1, 20, 1, 16000, 16)
	wav := Encode(pcm, 1, 16000, 16)

	// 在 fmt 后面、data 前面插入一个 LIST chunk
	listChunk := make([]byte, 8+4)
	copy(listChunk[0:4], []byte("LIST"))
	binary.LittleEndian.PutUint32(listChunk[4:8], 4)
	copy(listChunk[8:12], []byte("XXXX"))

	// 重新组装：RIFF/WAVE[12] + fmt[12:36] + LIST[12] + data[36:]
	fmtChunk := wav[12:36]
	dataChunk := wav[36:]
	out := make([]byte, 0, len(wav)+len(listChunk))
	out = append(out, []byte("RIFF")...)
	tmp := make([]byte, 4)
	binary.LittleEndian.PutUint32(tmp, uint32(len(wav)+len(listChunk)-8))
	out = append(out, tmp...)
	out = append(out, []byte("WAVE")...)
	out = append(out, fmtChunk...)
	out = append(out, listChunk...)
	out = append(out, dataChunk...)

	parsed, err := Parse(out)
	if err != nil {
		t.Fatalf("parse with extra chunk failed: %v", err)
	}
	if !bytes.Equal(parsed.Data, pcm) {
		t.Fatal("pcm data mismatch with extra chunk")
	}
}

func TestParse_NonPCMFormatValue(t *testing.T) {
	// format=2 (ADPCM) 应报错
	pcm := []byte{0, 0}
	wav := Encode(pcm, 1, 16000, 4)
	binary.LittleEndian.PutUint16(wav[20:22], 2)
	_, err := Parse(wav)
	if err == nil {
		t.Fatal("expected error for ADPCM format")
	}
	_ = errors.Is // 保留 errors 包引用
}
