package voice

import (
	"encoding/binary"
	"math"
	"testing"
)

func loudPCM(level float64) []byte {
	v := int16(level * 32767)
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(v))
	return b
}

func silentPCM() []byte {
	return make([]byte, 2)
}

func tone(level float64, ms, sampleRate int) []byte {
	n := sampleRate * ms / 1000
	b := make([]byte, n*2)
	v := int16(level * 32767)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	return b
}

func TestRMS(t *testing.T) {
	t.Parallel()
	if RMS(nil) != 0 {
		t.Fatal("nil pcm should be 0")
	}
	if RMS([]byte{0}) != 0 {
		t.Fatal("odd byte pcm should be 0")
	}
	r := RMS(loudPCM(1.0))
	if math.Abs(r-1.0) > 0.01 {
		t.Fatalf("RMS loud = %v", r)
	}
	if RMS(silentPCM()) != 0 {
		t.Fatal("silent pcm should be 0")
	}
}

func TestVAD_SustainedSilence(t *testing.T) {
	t.Parallel()
	v := &VAD{}
	pcm := tone(0, 500, 16000)
	if e := v.Push(pcm); e != EdgeNone {
		t.Fatalf("silence should not trigger edge, got %v", e)
	}
}

func TestVAD_SingleSpikeNoStart(t *testing.T) {
	t.Parallel()
	v := &VAD{}
	var pcm []byte
	pcm = append(pcm, tone(0, 100, 16000)...)
	pcm = append(pcm, tone(1.0, 20, 16000)...)
	pcm = append(pcm, tone(0, 100, 16000)...)
	if e := v.Push(pcm); e != EdgeNone {
		t.Fatalf("single spike should not trigger SpeechStart, got %v", e)
	}
}

func TestVAD_SustainedLoudOneStart(t *testing.T) {
	t.Parallel()
	v := &VAD{}
	pcm := tone(1.0, 200, 16000)
	if e := v.Push(pcm); e != SpeechStart {
		t.Fatalf("sustained loud should trigger SpeechStart, got %v", e)
	}
	for i := 0; i < 5; i++ {
		if e := v.Push(tone(1.0, 100, 16000)); e == SpeechStart {
			t.Fatal("continued loud should not re-emit SpeechStart")
		}
	}
}

func TestVAD_StopThenRearm(t *testing.T) {
	t.Parallel()
	v := &VAD{}
	if e := v.Push(tone(1.0, 200, 16000)); e != SpeechStart {
		t.Fatalf("expected SpeechStart, got %v", e)
	}
	var stop Edge
	for i := 0; i < 40; i++ {
		if e := v.Push(tone(0, 20, 16000)); e == SpeechStop {
			stop = e
			break
		}
	}
	if stop != SpeechStop {
		t.Fatal("sustained silence after speech should trigger SpeechStop")
	}
	if e := v.Push(tone(1.0, 200, 16000)); e != SpeechStart {
		t.Fatalf("loud after stop should re-arm SpeechStart, got %v", e)
	}
}

func TestVAD_SubFrameChunks(t *testing.T) {
	t.Parallel()
	v := &VAD{}
	loud := tone(1.0, 200, 16000)
	var starts int
	for i := 0; i < len(loud); i += 37 {
		end := i + 37
		if end > len(loud) {
			end = len(loud)
		}
		if e := v.Push(loud[i:end]); e == SpeechStart {
			starts++
		}
	}
	if starts != 1 {
		t.Fatalf("sub-frame chunks should yield exactly one SpeechStart, got %d", starts)
	}
}
