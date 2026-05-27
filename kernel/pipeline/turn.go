package pipeline

import (
	"context"
	"maps"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/maven/kernel/agent"
)

func mergePromptAndBlocks(prompt string, contentBlocks []model.ContentBlock) (string, []model.ContentBlock) {
	blocks := contentBlocks
	if len(contentBlocks) > 0 && strings.TrimSpace(prompt) != "" {
		blocks = make([]model.ContentBlock, 0, len(contentBlocks)+1)
		blocks = append(blocks, model.ContentBlock{Type: model.ContentBlockText, Text: prompt})
		blocks = append(blocks, contentBlocks...)
		prompt = ""
	}
	return prompt, blocks
}

func runResponseWithMetadata(ctx context.Context, rt agent.Runtime, prompt, sessionID string, contentBlocks []model.ContentBlock, metadata map[string]any) (*api.Response, error) {
	prompt, blocks := mergePromptAndBlocks(prompt, contentBlocks)
	req := api.Request{
		Prompt:        prompt,
		ContentBlocks: blocks,
		SessionID:     sessionID,
	}
	if len(metadata) > 0 {
		req.Metadata = maps.Clone(metadata)
	}
	return rt.Run(ctx, req)
}

func runStreamWithMetadata(ctx context.Context, rt agent.Runtime, prompt, sessionID string, contentBlocks []model.ContentBlock, metadata map[string]any) (<-chan api.StreamEvent, error) {
	prompt, blocks := mergePromptAndBlocks(prompt, contentBlocks)
	req := api.Request{
		Prompt:        prompt,
		ContentBlocks: blocks,
		SessionID:     sessionID,
	}
	if len(metadata) > 0 {
		req.Metadata = maps.Clone(metadata)
	}
	return rt.RunStream(ctx, req)
}
