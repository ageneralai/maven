package deepgram

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
)

// TTS streams linear16 PCM from Deepgram speak HTTP API (mono, 24 kHz).
type TTS struct {
	APIKey string
	Model  string
}

func (d *TTS) Synthesize(ctx context.Context, text string) (<-chan []byte, error) {
	t := strings.TrimSpace(text)
	if t == "" {
		ch := make(chan []byte)
		close(ch)
		return ch, nil
	}
	if strings.TrimSpace(d.APIKey) == "" {
		return nil, errors.New("voice: deepgram api key is empty")
	}
	model := strings.TrimSpace(d.Model)
	if model == "" {
		model = "aura-2-en-us"
	}
	q := url.Values{}
	q.Set("model", model)
	q.Set("encoding", "linear16")
	q.Set("sample_rate", "24000")
	q.Set("container", "none")
	endpoint := "https://api.deepgram.com/v1/speak?" + q.Encode()
	body, err := json.Marshal(map[string]string{"text": t})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+d.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("voice: deepgram tts http %d: %s", resp.StatusCode, strings.TrimSpace(string(slurp)))
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
