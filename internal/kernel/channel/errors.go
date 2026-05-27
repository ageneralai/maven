package channels

import (
	"errors"
	"fmt"
)

var ErrDeliveryFailed = errors.New("channel: delivery failed")

func WrapDeliveryFailed(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrDeliveryFailed, err)
}
