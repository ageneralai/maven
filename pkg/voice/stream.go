package voice

import (
	"context"
	"errors"
	"io"
)

// StreamHTTPBody reads body in bufSize chunks into a channel until EOF or ctx cancel.
func StreamHTTPBody(ctx context.Context, body io.ReadCloser, bufSize int) <-chan []byte {
	if bufSize <= 0 {
		bufSize = 8192
	}
	out := make(chan []byte, 16)
	go func() {
		defer close(out)
		defer func() { _ = body.Close() }()
		buf := make([]byte, bufSize)
		for {
			n, rerr := body.Read(buf)
			if n > 0 {
				cp := make([]byte, n)
				copy(cp, buf[:n])
				select {
				case <-ctx.Done():
					return
				case out <- cp:
				}
			}
			if rerr != nil {
				if errors.Is(rerr, io.EOF) {
					return
				}
				return
			}
		}
	}()
	return out
}
