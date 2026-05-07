package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const defaultReloadDebounce = 800 * time.Millisecond

// WatchConfigPath emits a signal on a debounced edge after the file at path
// changes (write/create/rename/remove). The parent directory is watched so
// atomic save (write temp + rename) is observed. stop is idempotent.
func WatchConfigPath(ctx context.Context, path string, debounce time.Duration) (<-chan struct{}, func()) {
	if debounce <= 0 {
		debounce = defaultReloadDebounce
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	dir := filepath.Dir(abs)
	base := filepath.Base(abs)
	out := make(chan struct{}, 1)
	var mu sync.Mutex
	stopped := false
	watcher, werr := fsnotify.NewWatcher()
	stop := func() {
		mu.Lock()
		defer mu.Unlock()
		if stopped {
			return
		}
		stopped = true
		if watcher != nil {
			_ = watcher.Close()
		}
	}
	if werr != nil {
		go func() {
			<-ctx.Done()
			stop()
		}()
		return out, stop
	}
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		go func() {
			<-ctx.Done()
		}()
		return out, stop
	}
	var (
		timerMu sync.Mutex
		timer   *time.Timer
	)
	emit := func() {
		select {
		case out <- struct{}{}:
		default:
		}
	}
	schedule := func() {
		timerMu.Lock()
		defer timerMu.Unlock()
		if timer != nil {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}
		timer = time.AfterFunc(debounce, emit)
	}
	go func() {
		defer stop()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-watcher.Events:
				if !ok {
					return
				}
				if ev.Name == "" {
					continue
				}
				if filepath.Base(ev.Name) != base {
					continue
				}
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
					schedule()
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()
	return out, stop
}

// FileModEpoch returns nanoseconds modtime for path or 0 if missing.
func FileModEpoch(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.ModTime().UnixNano()
}
