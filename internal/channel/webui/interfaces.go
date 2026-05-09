package webui

import chann "github.com/ageneralai/maven/internal/channel"

var _ chann.Channel = (*WebUIChannel)(nil)

var _ chann.StreamChannel = (*WebUIChannel)(nil)
