package matrix

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ageneralai/maven/internal/testutil"
	"github.com/ageneralai/maven/pkg/stringutil"
)

func TestGolden_matrixSendChunks(t *testing.T) {
	t.Parallel()
	content := strings.Repeat("x", matrixSendChunkSize+100)
	chunks := stringutil.ChunkBytes(content, matrixSendChunkSize)
	data, err := json.MarshalIndent(chunks, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	testutil.AssertGoldenFile(t, "matrix_send_chunks.json.golden", data)
}
