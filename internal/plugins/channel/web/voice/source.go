package voice

import (
	"context"

	"github.com/ageneralai/maven/internal/kernel/converse/adapter"
	"github.com/coder/websocket"
)

// wsPCMOpener reads binary websocket frames into a PCM stream.
func wsPCMOpener(conn *websocket.Conn) adapter.PCMOpener {
	return func(ctx context.Context) (<-chan []byte, error) {
		ch := make(chan []byte, 64)
		go func() {
			defer close(ch)
			for {
				typ, data, err := conn.Read(ctx)
				if err != nil {
					return
				}
				if typ != websocket.MessageBinary || len(data) == 0 {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case ch <- data:
				}
			}
		}()
		return ch, nil
	}
}
