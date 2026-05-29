package voice

import (
	"encoding/binary"
	"math"
	"time"
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

// Detector scores speech likelihood in [0,1] for one fixed analysis frame (s16le mono).
type Detector interface {
	Score(frame []byte) float64
}

// Energy scores via normalized RMS. Zero-dependency default detector.
type Energy struct{}

func (Energy) Score(frame []byte) float64 { return RMS(frame) }

type Edge int

const (
	EdgeNone Edge = iota
	SpeechStart
	SpeechStop
)

type vadState int

const (
	vadQuiet vadState = iota
	vadStarting
	vadSpeaking
	vadStopping
)

type VAD struct {
	Detector   Detector
	SampleRate int
	FrameMS    int
	StartHold  time.Duration
	StopHold   time.Duration
	Threshold  float64
	Smoothing  float64
	buf        []byte
	smoothed   float64
	state      vadState
	startCount int
	stopCount  int
}

func (v *VAD) Push(pcm []byte) Edge {
	v.buf = append(v.buf, pcm...)
	frameBytes := v.frameBytes()
	var edge Edge
	for len(v.buf) >= frameBytes {
		frame := v.buf[:frameBytes]
		v.buf = v.buf[frameBytes:]
		if e := v.processFrame(frame); e != EdgeNone && edge == EdgeNone {
			edge = e
		}
	}
	return edge
}

func (v *VAD) processFrame(frame []byte) Edge {
	raw := v.detector().Score(frame)
	v.smoothed += v.smoothing() * (raw - v.smoothed)
	thr := v.threshold()
	speaking := v.smoothed >= thr
	energetic := raw >= thr
	startFrames := v.startFrames()
	stopFrames := v.stopFrames()
	switch v.state {
	case vadQuiet:
		if speaking {
			v.state = vadStarting
			v.startCount = 1
		}
	case vadStarting:
		if !speaking {
			v.state = vadQuiet
			v.startCount = 0
		} else if energetic {
			v.startCount++
			if v.startCount >= startFrames {
				v.state = vadSpeaking
				v.startCount = 0
				return SpeechStart
			}
		}
	case vadSpeaking:
		if !speaking {
			v.state = vadStopping
			v.stopCount = 1
		}
	case vadStopping:
		if speaking {
			v.state = vadSpeaking
			v.stopCount = 0
		} else {
			v.stopCount++
			if v.stopCount >= stopFrames {
				v.state = vadQuiet
				v.stopCount = 0
				return SpeechStop
			}
		}
	}
	return EdgeNone
}

func (v *VAD) detector() Detector {
	if v.Detector == nil {
		return Energy{}
	}
	return v.Detector
}

func (v *VAD) sampleRate() int {
	if v.SampleRate == 0 {
		return 16000
	}
	return v.SampleRate
}

func (v *VAD) frameMS() int {
	if v.FrameMS == 0 {
		return 20
	}
	return v.FrameMS
}

func (v *VAD) frameBytes() int {
	return v.sampleRate() * v.frameMS() / 1000 * 2
}

func (v *VAD) startHold() time.Duration {
	if v.StartHold == 0 {
		return 120 * time.Millisecond
	}
	return v.StartHold
}

func (v *VAD) stopHold() time.Duration {
	if v.StopHold == 0 {
		return 250 * time.Millisecond
	}
	return v.StopHold
}

func (v *VAD) threshold() float64 {
	if v.Threshold == 0 {
		return DefaultVADThreshold
	}
	return v.Threshold
}

func (v *VAD) smoothing() float64 {
	if v.Smoothing == 0 {
		return 0.2
	}
	return v.Smoothing
}

func (v *VAD) startFrames() int {
	n := int(math.Round(float64(v.startHold()) / float64(time.Millisecond) / float64(v.frameMS())))
	if n < 1 {
		return 1
	}
	return n
}

func (v *VAD) stopFrames() int {
	n := int(math.Round(float64(v.stopHold()) / float64(time.Millisecond) / float64(v.frameMS())))
	if n < 1 {
		return 1
	}
	return n
}
