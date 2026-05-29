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

func TestVAD_Onset(t *testing.T) {
	t.Parallel()
	v := &VAD{}
	if v.Onset(silentPCM()) {
		t.Fatal("silence should not trigger onset")
	}
	if !v.Onset(loudPCM(1.0)) {
		t.Fatal("speech after silence should trigger onset")
	}
	if v.Onset(loudPCM(1.0)) {
		t.Fatal("continued speech should not re-trigger onset")
	}
	if v.Onset(silentPCM()) {
		t.Fatal("silence alone should not trigger onset")
	}
	if !v.Onset(loudPCM(1.0)) {
		t.Fatal("second onset after re-arm should fire")
	}
}

func TestVAD_DefaultThreshold(t *testing.T) {
	t.Parallel()
	v := &VAD{}
	if v.threshold() != DefaultVADThreshold {
		t.Fatalf("threshold = %v", v.threshold())
	}
}
