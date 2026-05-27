package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

// TTS streams PCM (pcm_24000) from ElevenLabs streaming endpoint.
type TTS struct {
	APIKey     string
	VoiceID    string
	HTTPClient *http.Client
}

type ttsReq struct {
	Text string `json:"text"`
}

func (e *TTS) Synthesize(ctx context.Context, text string) (<-chan []byte, error) {
	t := strings.TrimSpace(text)
	if t == "" {
		ch := make(chan []byte)
		close(ch)
		return ch, nil
	}
	if strings.TrimSpace(e.APIKey) == "" {
		return nil, errors.New("voice: elevenlabs api key is empty")
	}
	voice := strings.TrimSpace(e.VoiceID)
	if voice == "" {
		return nil, errors.New("voice: elevenlabs voice id is empty")
	}
	payload, err := json.Marshal(ttsReq{Text: t})
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s/stream", voice)
	q := url.Values{}
	q.Set("optimize_streaming_latency", "3")
	q.Set("output_format", "pcm_24000")
	endpoint := u + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("xi-api-key", e.APIKey)
	req.Header.Set("Content-Type", "application/json")
	hc := e.HTTPClient
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
		return nil, fmt.Errorf("voice: elevenlabs tts http %d: %s", resp.StatusCode, strings.TrimSpace(string(slurp)))
	}
	return pkgvoice.StreamHTTPBody(ctx, resp.Body, 8192), nil
}
