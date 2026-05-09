package voice

import (
	"github.com/ageneralai/maven/pkg/cartesia"
	"github.com/ageneralai/maven/pkg/deepgram"
	"github.com/ageneralai/maven/pkg/elevenlabs"
	"github.com/ageneralai/maven/pkg/openai"
	"github.com/ageneralai/maven/pkg/plugin"
)

// VoicePlugins returns speech provider plugins for registry composition.
func VoicePlugins() []plugin.Plugin {
	return []plugin.Plugin{
		cartesia.NewPlugin(),
		deepgram.NewPlugin(),
		elevenlabs.NewPlugin(),
		openai.NewPlugin(),
	}
}

// DefaultVoiceRegistry is used when no registry is injected (e.g. isolated tests).
func DefaultVoiceRegistry() *plugin.Registry {
	return plugin.NewRegistry(VoicePlugins()...)
}
