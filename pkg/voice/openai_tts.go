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

// OpenAITTS streams PCM audio from OpenAI speech API (chunked response body, 24 kHz mono).
type OpenAITTS struct {
	APIKey string
	Model  string
	Voice  string
}

type openAISpeechReq struct {
	Model          string `json:"model"`
	Voice          string `json:"voice"`
	Input          string `json:"input"`
	ResponseFormat string `json:"response_format"`
}

func (o *OpenAITTS) Synthesize(ctx context.Context, text string) (<-chan []byte, error) {
	t := strings.TrimSpace(text)
	if t == "" {
		ch := make(chan []byte)
		close(ch)
		return ch, nil
	}
	if strings.TrimSpace(o.APIKey) == "" {
		return nil, errors.New("voice: openai api key is empty")
	}
	model := strings.TrimSpace(o.Model)
	if model == "" {
		model = "tts-1"
	}
	voice := strings.TrimSpace(o.Voice)
	if voice == "" {
		voice = "alloy"
	}
	payload, err := json.Marshal(openAISpeechReq{
		Model:          model,
		Voice:          voice,
		Input:          t,
		ResponseFormat: "pcm",
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/audio/speech", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("voice: openai tts http %d: %s", resp.StatusCode, strings.TrimSpace(string(slurp)))
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
