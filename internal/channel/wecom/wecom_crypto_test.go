package wecom

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestWecomEncryptDecryptRoundTrip(t *testing.T) {
	key32 := make([]byte, 32)
	for i := range key32 {
		key32[i] = byte(i + 1)
	}
	encodingKey := strings.TrimRight(base64.StdEncoding.EncodeToString(key32), "=")
	plaintext := "hello-wecom-payload"
	receiveID := "recv-test-id"
	cipherB64, err := wecomEncrypt(encodingKey, receiveID, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	got, gotReceiveID, err := wecomDecrypt(encodingKey, receiveID, cipherB64)
	if err != nil {
		t.Fatal(err)
	}
	if got != plaintext {
		t.Fatalf("plaintext = %q, want %q", got, plaintext)
	}
	if gotReceiveID != receiveID {
		t.Fatalf("receiveID = %q, want %q", gotReceiveID, receiveID)
	}
}
