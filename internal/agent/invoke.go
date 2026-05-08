package agent

import (
	"context"
	"maps"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/model"
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

// RunResponse executes a non-streaming turn with the same ContentBlocks + prompt merge rules as the gateway pipeline.
func RunResponse(ctx context.Context, rt Runtime, prompt, sessionID string, contentBlocks []model.ContentBlock) (*api.Response, error) {
	return RunResponseWithMetadata(ctx, rt, prompt, sessionID, contentBlocks, nil)
}

// RunResponseWithMetadata is like RunResponse but attaches api.Request.Metadata (e.g. slash prepends).
func RunResponseWithMetadata(ctx context.Context, rt Runtime, prompt, sessionID string, contentBlocks []model.ContentBlock, metadata map[string]any) (*api.Response, error) {
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

// RunText returns the assistant text output, if any.
func RunText(ctx context.Context, rt Runtime, prompt, sessionID string, contentBlocks []model.ContentBlock) (string, error) {
	resp, err := RunResponse(ctx, rt, prompt, sessionID, contentBlocks)
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Result == nil {
		return "", nil
	}
	return resp.Result.Output, nil
}

// RunStream starts streaming with the same ContentBlocks + prompt merge rules as RunResponse.
func RunStream(ctx context.Context, rt Runtime, prompt, sessionID string, contentBlocks []model.ContentBlock) (<-chan api.StreamEvent, error) {
	return RunStreamWithMetadata(ctx, rt, prompt, sessionID, contentBlocks, nil)
}

// RunStreamWithMetadata is like RunStream but attaches api.Request.Metadata.
func RunStreamWithMetadata(ctx context.Context, rt Runtime, prompt, sessionID string, contentBlocks []model.ContentBlock, metadata map[string]any) (<-chan api.StreamEvent, error) {
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
