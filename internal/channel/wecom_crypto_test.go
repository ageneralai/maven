package channel

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestWecomSignatureGolden(t *testing.T) {
	got := wecomSignature("t1", "time1", "n1", "data1")
	want := "9ed1f1192b806800384a4bbf59377766edec0aea"
	if got != want {
		t.Fatalf("signature = %q, want %q (sorted join data1n1t1time1)", got, want)
	}
	if got2 := wecomSignature("t1", "time1", "n1", "data1"); got2 != got {
		t.Fatal("signature not deterministic")
	}
}

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
	if cipherB64 == "" {
		t.Fatal("empty ciphertext")
	}
	msg, rid, err := wecomDecrypt(encodingKey, receiveID, cipherB64)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if msg != plaintext {
		t.Fatalf("plaintext = %q, want %q", msg, plaintext)
	}
	if rid != receiveID {
		t.Fatalf("receiveID = %q, want %q", rid, receiveID)
	}
}

func TestWecomEncryptDecryptRoundTrip_EmptyExpectedReceiveIDSkipsCheck(t *testing.T) {
	key32 := make([]byte, 32)
	for i := range key32 {
		key32[i] = 42
	}
	encodingKey := strings.TrimRight(base64.StdEncoding.EncodeToString(key32), "=")
	receiveID := "any-rid"
	s, err := wecomEncrypt(encodingKey, receiveID, "x")
	if err != nil {
		t.Fatal(err)
	}
	msg, rid, err := wecomDecrypt(encodingKey, "", s)
	if err != nil {
		t.Fatal(err)
	}
	if msg != "x" || rid != receiveID {
		t.Fatalf("got msg=%q rid=%q", msg, rid)
	}
}
