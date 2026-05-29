package converse

import "context"

// Tee fans out in to n identical output channels.
func Tee[T any](ctx context.Context, in <-chan T, n int) []<-chan T {
	if n <= 0 {
		return nil
	}
	outs := make([]chan T, n)
	for i := range outs {
		outs[i] = make(chan T, 64)
	}
	go func() {
		defer func() {
			for _, ch := range outs {
				close(ch)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				for _, ch := range outs {
					select {
					case <-ctx.Done():
						return
					case ch <- v:
					}
				}
			}
		}
	}()
	readOnly := make([]<-chan T, n)
	for i, ch := range outs {
		readOnly[i] = ch
	}
	return readOnly
}
