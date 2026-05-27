package responses

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func writeSSE(w http.ResponseWriter, flusher http.Flusher, eventType string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, b); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func writeDone(w http.ResponseWriter, flusher http.Flusher) error {
	if _, err := fmt.Fprint(w, "data: [DONE]\n\n"); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
