package deepgram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"
)

// STT streams PCM16 LE mono audio to Deepgram live and emits final transcripts.
type STT struct {
	APIKey      string
	Model       string
	Endpointing string
}

func (d *STT) Transcribe(ctx context.Context, audio <-chan []byte) (<-chan string, error) {
	if strings.TrimSpace(d.APIKey) == "" {
		return nil, errors.New("voice: deepgram api key is empty")
	}
	model := strings.TrimSpace(d.Model)
	if model == "" {
		model = "nova-2"
	}
	ep := strings.TrimSpace(d.Endpointing)
	if ep == "" {
		ep = "400"
	}
	q := url.Values{}
	q.Set("model", model)
	q.Set("encoding", "linear16")
	q.Set("sample_rate", "16000")
	q.Set("channels", "1")
	q.Set("punctuate", "true")
	q.Set("interim_results", "false")
	q.Set("endpointing", ep)
	u := "wss://api.deepgram.com/v1/listen?" + q.Encode()
	out := make(chan string, 8)
	conn, _, err := websocket.Dial(ctx, u, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Token " + d.APIKey}},
	})
	if err != nil {
		return nil, fmt.Errorf("voice: deepgram dial: %w", err)
	}
	go func() {
		defer close(out)
		defer func() { _ = conn.CloseNow() }()
		readErr := make(chan error, 1)
		go func() {
			readErr <- d.readLoop(ctx, conn, out)
		}()
		for {
			select {
			case <-ctx.Done():
				_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"type":"CloseStream"}`))
				<-readErr
				return
			case chunk, ok := <-audio:
				if !ok {
					_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"type":"CloseStream"}`))
					<-readErr
					return
				}
				if len(chunk) == 0 {
					continue
				}
				if werr := conn.Write(ctx, websocket.MessageBinary, chunk); werr != nil {
					return
				}
			}
		}
	}()
	return out, nil
}

func (d *STT) readLoop(ctx context.Context, conn *websocket.Conn, out chan<- string) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		typ, data, err := conn.Read(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if typ != websocket.MessageText {
			continue
		}
		var wrap map[string]json.RawMessage
		if err := json.Unmarshal(data, &wrap); err != nil {
			continue
		}
		t := ""
		if raw, ok := wrap["type"]; ok {
			_ = json.Unmarshal(raw, &t)
		}
		if t == "Metadata" {
			return nil
		}
		isFinal := false
		if raw, ok := wrap["is_final"]; ok {
			_ = json.Unmarshal(raw, &isFinal)
		}
		speechFinal := false
		if raw, ok := wrap["speech_final"]; ok {
			_ = json.Unmarshal(raw, &speechFinal)
		}
		if !isFinal && !speechFinal {
			continue
		}
		tr := extractTranscript(data)
		tr = strings.TrimSpace(tr)
		if tr == "" {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- tr:
		}
	}
}

type dgAlt struct {
	Transcript string `json:"transcript"`
}

type dgChannel struct {
	Alternatives []dgAlt `json:"alternatives"`
}

type dgResultMsg struct {
	Channel dgChannel `json:"channel"`
}

func extractTranscript(data []byte) string {
	var m dgResultMsg
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if len(m.Channel.Alternatives) == 0 {
		return ""
	}
	return m.Channel.Alternatives[0].Transcript
}
