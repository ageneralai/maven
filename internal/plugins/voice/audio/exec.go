package audio

import (
	"context"
	"errors"
	"io"
	"os/exec"
)

// ExecCapture runs command+args and streams stdout as raw PCM chunks.
type ExecCapture struct {
	Command string
	Args    []string
}

func (c *ExecCapture) Capture(ctx context.Context) (<-chan []byte, error) {
	if c == nil || c.Command == "" {
		return nil, errors.New("audio: capture command is empty")
	}
	cmd := exec.CommandContext(ctx, c.Command, c.Args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	out := make(chan []byte, 32)
	go func() {
		defer close(out)
		defer func() { _ = cmd.Wait() }()
		buf := make([]byte, 4096)
		for {
			n, rerr := stdout.Read(buf)
			if n > 0 {
				cp := make([]byte, n)
				copy(cp, buf[:n])
				select {
				case <-ctx.Done():
					_ = cmd.Process.Kill()
					return
				case out <- cp:
				}
			}
			if rerr != nil {
				if errors.Is(rerr, io.EOF) {
					return
				}
				if ctx.Err() != nil {
					return
				}
				return
			}
		}
	}()
	return out, nil
}

// ExecPlayback pipes PCM chunks to command+args stdin.
type ExecPlayback struct {
	Command string
	Args    []string
}

func (p *ExecPlayback) Play(ctx context.Context, pcm <-chan []byte) error {
	if p == nil || p.Command == "" {
		return errors.New("audio: playback command is empty")
	}
	cmd := exec.CommandContext(ctx, p.Command, p.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	writeDone := make(chan error, 1)
	go func() {
		defer close(writeDone)
		for {
			select {
			case <-ctx.Done():
				_ = stdin.Close()
				return
			case chunk, ok := <-pcm:
				if !ok {
					_ = stdin.Close()
					return
				}
				if len(chunk) == 0 {
					continue
				}
				if _, werr := stdin.Write(chunk); werr != nil {
					writeDone <- werr
					return
				}
			}
		}
	}()
	waitErr := cmd.Wait()
	select {
	case werr := <-writeDone:
		if werr != nil {
			return werr
		}
	default:
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return waitErr
}
