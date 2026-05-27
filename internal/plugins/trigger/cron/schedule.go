package cron

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/adhocore/gronx"
)

type Schedule interface {
	Next(after time.Time) (time.Time, error)
	Validate() error
}

type CronSchedule struct {
	Expr string
}

func (s CronSchedule) Next(after time.Time) (time.Time, error) {
	return gronx.NextTickAfter(s.Expr, after, false)
}

func (s CronSchedule) Validate() error {
	if strings.TrimSpace(s.Expr) == "" {
		return errors.New("cron schedule: expr is required")
	}
	if !gronx.IsValid(strings.TrimSpace(s.Expr)) {
		return fmt.Errorf("cron schedule: invalid expr %q", s.Expr)
	}
	return nil
}

type EverySchedule struct {
	Interval time.Duration
}

func (s EverySchedule) Next(after time.Time) (time.Time, error) {
	if s.Interval <= 0 {
		return time.Time{}, nil
	}
	return after.Add(s.Interval), nil
}

func (s EverySchedule) Validate() error {
	if s.Interval <= 0 {
		return errors.New("every schedule: interval must be positive")
	}
	return nil
}

type AtSchedule struct {
	At time.Time
}

func (s AtSchedule) Next(after time.Time) (time.Time, error) {
	if s.At.UnixMilli() >= after.UnixMilli() {
		return s.At, nil
	}
	return time.Time{}, nil
}

func (s AtSchedule) Validate() error {
	if s.At.IsZero() {
		return errors.New("at schedule: time is required")
	}
	return nil
}

func IsAtSchedule(s Schedule) bool {
	_, ok := s.(AtSchedule)
	return ok
}

type scheduleJSON struct {
	Kind    string `json:"kind"`
	Expr    string `json:"expr,omitempty"`
	EveryMs int64  `json:"everyMs,omitempty"`
	AtMs    int64  `json:"atMs,omitempty"`
}

func marshalSchedule(s Schedule) ([]byte, error) {
	var raw scheduleJSON
	switch v := s.(type) {
	case CronSchedule:
		raw = scheduleJSON{Kind: "cron", Expr: v.Expr}
	case EverySchedule:
		raw = scheduleJSON{Kind: "every", EveryMs: v.Interval.Milliseconds()}
	case AtSchedule:
		raw = scheduleJSON{Kind: "at", AtMs: v.At.UnixMilli()}
	default:
		if s == nil {
			return []byte("null"), nil
		}
		return nil, fmt.Errorf("marshal schedule: unsupported type %T", s)
	}
	return json.Marshal(raw)
}

func unmarshalSchedule(data []byte) (Schedule, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, errors.New("schedule is required")
	}
	var raw scheduleJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	switch raw.Kind {
	case "cron":
		return CronSchedule{Expr: raw.Expr}, nil
	case "every":
		return EverySchedule{Interval: time.Duration(raw.EveryMs) * time.Millisecond}, nil
	case "at":
		return AtSchedule{At: time.UnixMilli(raw.AtMs)}, nil
	default:
		return nil, fmt.Errorf("unknown schedule kind %q", raw.Kind)
	}
}
