package cartesia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	pkgvoice "github.com/ageneralai/maven/internal/kernel/voice"
)

// TTS streams raw PCM (pcm_s16le) from Cartesia TTS bytes API.
type TTS struct {
	APIKey     string
	ModelID    string
	VoiceID    string
	Version    string
	HTTPClient *http.Client
}

type ttsRequest struct {
	ModelID      string            `json:"model_id"`
	Transcript   string            `json:"transcript"`
	Voice        map[string]string `json:"voice"`
	OutputFormat map[string]any    `json:"output_format"`
}

func (c *TTS) Synthesize(ctx context.Context, text string) (<-chan []byte, error) {
	t := strings.TrimSpace(text)
	if t == "" {
		ch := make(chan []byte)
		close(ch)
		return ch, nil
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, errors.New("voice: cartesia api key is empty")
	}
	voiceID := strings.TrimSpace(c.VoiceID)
	if voiceID == "" {
		return nil, errors.New("voice: cartesia voice id is empty")
	}
	model := strings.TrimSpace(c.ModelID)
	if model == "" {
		model = "sonic-2"
	}
	ver := strings.TrimSpace(c.Version)
	if ver == "" {
		ver = "2025-04-16"
	}
	body, err := json.Marshal(ttsRequest{
		ModelID:    model,
		Transcript: t,
		Voice: map[string]string{
			"mode": "id",
			"id":   voiceID,
		},
		OutputFormat: map[string]any{
			"container":   "raw",
			"encoding":    "pcm_s16le",
			"sample_rate": 24000,
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.cartesia.ai/tts/bytes", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Cartesia-Version", ver)
	req.Header.Set("Content-Type", "application/json")
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("voice: cartesia tts http %d: %s", resp.StatusCode, strings.TrimSpace(string(slurp)))
	}
	return pkgvoice.StreamHTTPBody(ctx, resp.Body, 8192), nil
}
