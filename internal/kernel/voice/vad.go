package voice

import (
	"encoding/binary"
	"math"
)

const DefaultVADThreshold = 0.01

func RMS(pcm []byte) float64 {
	samples := len(pcm) / 2
	if samples == 0 {
		return 0
	}
	var sum float64
	for i := 0; i+1 < len(pcm); i += 2 {
		v := int16(binary.LittleEndian.Uint16(pcm[i:])) //nolint:gosec
		x := float64(v) / 32768
		sum += x * x
	}
	return math.Sqrt(sum / float64(samples))
}

// VAD detects speech onset after silence (armed→speaking transition).
type VAD struct {
	Threshold float64
	armed     bool
	seen      bool
}

func (v *VAD) threshold() float64 {
	if v.Threshold == 0 {
		return DefaultVADThreshold
	}
	return v.Threshold
}

func (v *VAD) Onset(pcm []byte) bool {
	if !v.seen {
		v.seen = true
		v.armed = true
	}
	speaking := RMS(pcm) > v.threshold()
	if speaking && v.armed {
		v.armed = false
		return true
	}
	if !speaking {
		v.armed = true
	}
	return false
}
