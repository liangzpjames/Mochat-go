package http

import (
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

const durationProbeLimit = 16 << 20

func probeAudioDuration(reader io.Reader, contentType string) (int64, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, durationProbeLimit))
	if err != nil {
		return 0, err
	}
	if len(payload) < 12 {
		return 0, errors.New("audio header is incomplete")
	}
	if strings.EqualFold(strings.TrimSpace(contentType), "audio/wav") || (len(payload) >= 12 && string(payload[:4]) == "RIFF" && string(payload[8:12]) == "WAVE") {
		return wavDuration(payload)
	}
	if strings.EqualFold(strings.TrimSpace(contentType), "audio/mpeg") || string(payload[:3]) == "ID3" {
		return mp3Duration(payload)
	}
	return 0, errors.New("audio duration format is not supported")
}

func wavDuration(payload []byte) (int64, error) {
	if len(payload) < 12 || string(payload[:4]) != "RIFF" || string(payload[8:12]) != "WAVE" {
		return 0, errors.New("invalid WAV header")
	}
	var sampleRate, channels, bitsPerSample uint32
	var dataBytes uint64
	for offset := 12; offset+8 <= len(payload); {
		chunkID := string(payload[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(payload[offset+4 : offset+8]))
		offset += 8
		if chunkSize < 0 || offset+chunkSize > len(payload) {
			return 0, errors.New("invalid WAV chunk")
		}
		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return 0, errors.New("invalid WAV fmt chunk")
			}
			channels = uint32(binary.LittleEndian.Uint16(payload[offset+2 : offset+4]))
			sampleRate = binary.LittleEndian.Uint32(payload[offset+4 : offset+8])
			bitsPerSample = uint32(binary.LittleEndian.Uint16(payload[offset+14 : offset+16]))
		case "data":
			dataBytes = uint64(chunkSize)
		}
		offset += chunkSize
		if chunkSize%2 == 1 {
			offset++
		}
	}
	if sampleRate == 0 || channels == 0 || bitsPerSample == 0 || dataBytes == 0 {
		return 0, errors.New("WAV duration metadata is incomplete")
	}
	denominator := uint64(sampleRate) * uint64(channels) * uint64(bitsPerSample) / 8
	if denominator == 0 {
		return 0, errors.New("WAV duration denominator is invalid")
	}
	seconds := int64(dataBytes / denominator)
	if seconds == 0 {
		seconds = 1
	}
	return seconds, nil
}

func mp3Duration(payload []byte) (int64, error) {
	offset := 0
	if len(payload) >= 10 && string(payload[:3]) == "ID3" {
		offset = 10 + int(payload[6]&0x7f)<<21 + int(payload[7]&0x7f)<<14 + int(payload[8]&0x7f)<<7 + int(payload[9]&0x7f)
	}
	var frames int64
	var sampleRate int64
	var samplesPerFrame int64
	for offset+4 <= len(payload) {
		header := binary.BigEndian.Uint32(payload[offset : offset+4])
		if header&0xffe00000 != 0xffe00000 {
			offset++
			continue
		}
		version := (header >> 19) & 0x3
		layer := (header >> 17) & 0x3
		bitrateIndex := (header >> 12) & 0xf
		sampleRateIndex := (header >> 10) & 0x3
		padding := (header >> 9) & 0x1
		if layer != 1 || version == 1 || bitrateIndex == 0 || bitrateIndex == 15 || sampleRateIndex == 3 {
			offset++
			continue
		}
		bitrates := []int64{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
		sampleRates := []int64{44100, 48000, 32000, 0}
		sampleRate = sampleRates[sampleRateIndex]
		if version == 2 {
			sampleRate /= 2
		} else if version == 0 {
			sampleRate /= 4
		}
		if version == 3 {
			samplesPerFrame = 1152
		} else {
			samplesPerFrame = 576
		}
		frameLength := int((144 * bitrates[bitrateIndex] * 1000) / sampleRate)
		if version != 3 {
			frameLength = int((72 * bitrates[bitrateIndex] * 1000) / sampleRate)
		}
		frameLength += int(padding)
		if frameLength <= 0 || offset+frameLength > len(payload) {
			break
		}
		frames++
		offset += frameLength
	}
	if frames == 0 || sampleRate == 0 || samplesPerFrame == 0 {
		return 0, errors.New("MP3 duration metadata is incomplete")
	}
	seconds := frames * samplesPerFrame / sampleRate
	if seconds == 0 {
		seconds = 1
	}
	return seconds, nil
}
