package slash

// PostAction is a gateway-internal post-turn effect set by slash handlers.
type PostAction interface {
	postAction()
}

// CompactRotateAction rotates the chat session after compact and optionally acks the user.
type CompactRotateAction struct {
	ResponseMode CompactResponseMode
}

func (CompactRotateAction) postAction() {}
