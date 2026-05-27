package channel

import (
	"errors"
	"testing"
)

func TestWrapDeliveryFailed(t *testing.T) {
	if WrapDeliveryFailed(nil) != nil {
		t.Fatal("nil in nil out")
	}
	root := errors.New("telegram timeout")
	wrapped := WrapDeliveryFailed(root)
	if !errors.Is(wrapped, ErrDeliveryFailed) {
		t.Fatalf("want ErrDeliveryFailed, got %v", wrapped)
	}
	if !errors.Is(wrapped, root) {
		t.Fatalf("want root cause preserved, got %v", wrapped)
	}
}
