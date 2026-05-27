package feishu

import (
	"encoding/json"
	"testing"

	"github.com/ageneralai/maven/internal/testutil"
)

func TestGolden_feishuTextMessagePayload(t *testing.T) {
	t.Parallel()
	data, err := feishuTextMessagePayload("chat_123", "hello")
	if err != nil {
		t.Fatal(err)
	}
	var norm map[string]any
	if err := json.Unmarshal(data, &norm); err != nil {
		t.Fatal(err)
	}
	pretty, err := json.MarshalIndent(norm, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	pretty = append(pretty, '\n')
	testutil.AssertGoldenFile(t, "feishu_text_outbound.json.golden", pretty)
}
