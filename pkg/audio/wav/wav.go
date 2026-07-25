// Package wav 提供标准 RIFF/WAVE (PCM, format=1) 音频的解析、编码与拼接能力。
// Concat 要求所有输入格式一致，不做重采样。
package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

type PCM struct {
	Data          []byte
	NumChannels   uint16
	SampleRate    uint32
	BitsPerSample uint16
}

func Parse(data []byte) (*PCM, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("wav too short: len=%d", len(data))
	}
	if !bytes.Equal(data[0:4], []byte("RIFF")) {
		return nil, fmt.Errorf("not a riff wav file: magic=%q", data[0:4])
	}
	if !bytes.Equal(data[8:12], []byte("WAVE")) {
		return nil, fmt.Errorf("not a wave file: magic=%q", data[8:12])
	}

	out := &PCM{}
	off := 12
	for off+8 <= len(data) {
		chunkID := string(data[off : off+4])
		chunkSize := binary.LittleEndian.Uint32(data[off+4 : off+8])
		bodyStart := off + 8
		bodyEnd := bodyStart + int(chunkSize)
		actualEnd := bodyEnd

		if bodyEnd > len(data) {
			remaining := len(data) - bodyStart
			switch chunkID {
			case "data":
				if remaining > 0 {
					actualEnd = len(data)
				} else {
					return nil, fmt.Errorf("wav data chunk missing: want=%d have=0", chunkSize)
				}
			default:
				return nil, fmt.Errorf("wav chunk %q truncated (want=%d have=%d)",
					chunkID, bodyEnd, len(data))
			}
		}

		body := data[bodyStart:actualEnd]

		switch chunkID {
		case "fmt ":
			if len(body) < 16 {
				return nil, fmt.Errorf("wav fmt chunk too short: len=%d", len(body))
			}
			format := binary.LittleEndian.Uint16(body[0:2])
			if format != 1 {
				return nil, fmt.Errorf("only PCM wav (format=1) supported, got format=%d", format)
			}
			out.NumChannels = binary.LittleEndian.Uint16(body[2:4])
			out.SampleRate = binary.LittleEndian.Uint32(body[4:8])
			out.BitsPerSample = binary.LittleEndian.Uint16(body[14:16])
		case "data":
			out.Data = body
		}

		off = bodyEnd + int(chunkSize&1)
	}

	if out.Data == nil {
		return nil, errors.New("wav missing data chunk")
	}
	if out.NumChannels == 0 || out.SampleRate == 0 || out.BitsPerSample == 0 {
		return nil, errors.New("wav missing or unsupported fmt chunk")
	}
	return out, nil
}

func Encode(pcm []byte, numChannels uint16, sampleRate uint32, bitsPerSample uint16) []byte {
	bytesPerSample := int(bitsPerSample / 8)
	byteRate := sampleRate * uint32(numChannels) * uint32(bytesPerSample)
	blockAlign := numChannels * uint16(bytesPerSample)
	dataSize := uint32(len(pcm))

	out := make([]byte, 44+len(pcm))
	copy(out[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(out[4:8], 36+dataSize)
	copy(out[8:12], []byte("WAVE"))
	copy(out[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(out[16:20], 16) // fmt chunk size
	binary.LittleEndian.PutUint16(out[20:22], 1)  // PCM
	binary.LittleEndian.PutUint16(out[22:24], numChannels)
	binary.LittleEndian.PutUint32(out[24:28], sampleRate)
	binary.LittleEndian.PutUint32(out[28:32], byteRate)
	binary.LittleEndian.PutUint16(out[32:34], blockAlign)
	binary.LittleEndian.PutUint16(out[34:36], bitsPerSample)
	copy(out[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(out[40:44], dataSize)
	copy(out[44:], pcm)
	return out
}

func ConcatPCMs(parts []*PCM) ([]byte, *PCM, error) {
	if len(parts) == 0 {
		return nil, nil, errors.New("no pcm parts to concat")
	}

	head := parts[0]
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		if p == nil {
			return nil, nil, fmt.Errorf("pcm part %d is nil", i)
		}
		if p.NumChannels != head.NumChannels ||
			p.SampleRate != head.SampleRate ||
			p.BitsPerSample != head.BitsPerSample {
			return nil, nil, fmt.Errorf(
				"pcm part %d format incompatible (ch=%d sr=%d bits=%d) with head (ch=%d sr=%d bits=%d)",
				i, p.NumChannels, p.SampleRate, p.BitsPerSample,
				head.NumChannels, head.SampleRate, head.BitsPerSample,
			)
		}
	}

	totalLen := 0
	for _, p := range parts {
		totalLen += len(p.Data)
	}
	merged := make([]byte, 0, totalLen)
	for _, p := range parts {
		merged = append(merged, p.Data...)
	}

	wav := Encode(merged, head.NumChannels, head.SampleRate, head.BitsPerSample)
	mergedPCM := &PCM{
		Data:          merged,
		NumChannels:   head.NumChannels,
		SampleRate:    head.SampleRate,
		BitsPerSample: head.BitsPerSample,
	}
	return wav, mergedPCM, nil
}

func Concat(parts [][]byte) ([]byte, error) {
	if len(parts) == 0 {
		return nil, errors.New("no wav parts to concat")
	}
	pcms := make([]*PCM, 0, len(parts))
	for i, p := range parts {
		pcm, err := Parse(p)
		if err != nil {
			return nil, fmt.Errorf("parse wav part %d failed: %w", i, err)
		}
		pcms = append(pcms, pcm)
	}
	wav, _, err := ConcatPCMs(pcms)
	return wav, err
}
