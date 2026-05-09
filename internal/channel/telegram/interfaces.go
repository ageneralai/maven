package telegram

import chann "github.com/ageneralai/maven/internal/channel"

var (
	_ chann.Channel             = (*TelegramChannel)(nil)
	_ chann.StreamChannel       = (*TelegramChannel)(nil)
	_ chann.InboundPreprocessor = (*TelegramChannel)(nil)
)
