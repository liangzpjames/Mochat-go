package http

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestProbeAudioDurationReadsWAVPCMHeader(t *testing.T) {
	const seconds = 7
	const sampleRate = 8000
	const channels = 1
	const bits = 16
	dataSize := sampleRate * channels * bits / 8 * seconds
	payload := make([]byte, 44+dataSize)
	copy(payload[:4], "RIFF")
	binary.LittleEndian.PutUint32(payload[4:8], uint32(len(payload)-8))
	copy(payload[8:12], "WAVE")
	copy(payload[12:16], "fmt ")
	binary.LittleEndian.PutUint32(payload[16:20], 16)
	binary.LittleEndian.PutUint16(payload[20:22], 1)
	binary.LittleEndian.PutUint16(payload[22:24], channels)
	binary.LittleEndian.PutUint32(payload[24:28], sampleRate)
	binary.LittleEndian.PutUint32(payload[28:32], sampleRate*channels*bits/8)
	binary.LittleEndian.PutUint16(payload[32:34], channels*bits/8)
	binary.LittleEndian.PutUint16(payload[34:36], bits)
	copy(payload[36:40], "data")
	binary.LittleEndian.PutUint32(payload[40:44], uint32(dataSize))

	got, err := probeAudioDuration(bytes.NewReader(payload), "audio/wav")
	if err != nil {
		t.Fatal(err)
	}
	if got != seconds {
		t.Fatalf("duration=%d, want %d", got, seconds)
	}
}
