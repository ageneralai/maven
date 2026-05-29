package audio

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	androidMicSource = "OpenSL_ES_source"
	androidMicModule = "module-sles-source"
	androidSinkLoad  = "module-aaudio-sink"
)

// ensureAndroidPulse reconciles PulseAudio before CLI voice capture on Android.
// Idempotent: probes pactl, loads the mic source when missing, restarts when wedged.
func ensureAndroidPulse(ctx context.Context, run Runner) error {
	if run == nil {
		run = defaultRunner
	}
	if err := probePulse(ctx, run); err == nil {
		if err := ensureMicSource(ctx, run); err == nil {
			return nil
		}
	}
	if err := restartAndroidPulse(ctx, run); err != nil {
		return &PulseUnavailableError{Err: fmt.Errorf("android pulse restart: %w", err)}
	}
	if err := waitForPulse(ctx, run); err != nil {
		return &PulseUnavailableError{Err: fmt.Errorf("android pulse after restart: %w", err)}
	}
	if err := ensureMicSource(ctx, run); err != nil {
		return &PulseUnavailableError{Err: fmt.Errorf("android mic source: %w", err)}
	}
	return nil
}

func probePulse(ctx context.Context, run Runner) error {
	_, err := run(ctx, "pactl", "info")
	return err
}

func ensureMicSource(ctx context.Context, run Runner) error {
	out, err := run(ctx, "pactl", "list", "short", "sources")
	if err != nil {
		return err
	}
	if sourcePresent(out, androidMicSource) {
		return nil
	}
	_, err = run(ctx, "pactl", "load-module", androidMicModule)
	return err
}

func restartAndroidPulse(ctx context.Context, run Runner) error {
	_, _ = run(ctx, "pulseaudio", "-k")
	_, _ = run(ctx, "killall", "pulseaudio")
	_, _ = run(ctx, "sh", "-c", `rm -rf "$PREFIX/tmp/pulse-"* 2>/dev/null; true`)
	if err := sleep(ctx, 2*time.Second); err != nil {
		return err
	}
	_, err := run(ctx, "pulseaudio", "--start", "--exit-idle-time=-1", "--load="+androidSinkLoad)
	return err
}

func waitForPulse(ctx context.Context, run Runner) error {
	var lastErr error
	for range 10 {
		if err := probePulse(ctx, run); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if err := sleep(ctx, 500*time.Millisecond); err != nil {
			return err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("daemon not ready")
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
