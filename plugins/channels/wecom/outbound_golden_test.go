package wecom

import (
	"encoding/json"
	"testing"

	"github.com/ageneralai/maven/kernel/config"
	"github.com/ageneralai/maven/internal/testutil"
)

func TestGolden_wecomMarkdownSendPayload(t *testing.T) {
	t.Parallel()
	body, err := wecomMarkdownSendPayload("pong")
	if err != nil {
		t.Fatal(err)
	}
	var norm map[string]any
	if err := json.Unmarshal(body, &norm); err != nil {
		t.Fatal(err)
	}
	pretty, err := json.MarshalIndent(norm, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	pretty = append(pretty, '\n')
	testutil.AssertGoldenFile(t, "wecom_markdown_send.json.golden", pretty)
}

func TestGolden_wecomEncryptedReplyEnvelope(t *testing.T) {
	ch, _ := newTestWeComChannel(t, config.WeComConfig{
		Token:          "verify-token",
		EncodingAESKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
		ReceiveID:      "recv-id-1",
	})
	body, err := ch.buildEncryptedReply("1739000000", "nonce-golden", "recv-id-1", "success")
	if err != nil {
		t.Fatal(err)
	}
	var norm map[string]any
	if err := json.Unmarshal(body, &norm); err != nil {
		t.Fatal(err)
	}
	delete(norm, "encrypt")
	delete(norm, "msgsignature")
	delete(norm, "MsgSignature")
	delete(norm, "msg_signature")
	pretty, err := json.MarshalIndent(norm, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	pretty = append(pretty, '\n')
	testutil.AssertGoldenFile(t, "wecom_reply_envelope.json.golden", pretty)
}
