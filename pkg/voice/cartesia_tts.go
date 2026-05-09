package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CartesiaTTS streams MP3 from Cartesia TTS bytes API (https://docs.cartesia.ai/2024-11-13/api-reference/tts/bytes).
type CartesiaTTS struct {
	APIKey  string
	ModelID string
	VoiceID string
	// Version is the Cartesia-Version header (e.g. 2025-04-16). See API versioning in Cartesia docs.
	Version string
}

type cartesiaTTSRequest struct {
	ModelID      string                 `json:"model_id"`
	Transcript   string                 `json:"transcript"`
	Voice        map[string]string      `json:"voice"`
	OutputFormat map[string]interface{} `json:"output_format"`
}

func (c *CartesiaTTS) Synthesize(ctx context.Context, text string) (<-chan []byte, error) {
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
	body, err := json.Marshal(cartesiaTTSRequest{
		ModelID:    model,
		Transcript: t,
		Voice: map[string]string{
			"mode": "id",
			"id":   voiceID,
		},
		OutputFormat: map[string]interface{}{
			"container":   "mp3",
			"sample_rate": 44100,
			"bit_rate":    128000,
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("voice: cartesia tts http %d: %s", resp.StatusCode, strings.TrimSpace(string(slurp)))
	}
	out := make(chan []byte, 16)
	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()
		buf := make([]byte, 8192)
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				cp := make([]byte, n)
				copy(cp, buf[:n])
				select {
				case <-ctx.Done():
					return
				case out <- cp:
				}
			}
			if rerr != nil {
				if errors.Is(rerr, io.EOF) {
					return
				}
				return
			}
		}
	}()
	return out, nil
}
