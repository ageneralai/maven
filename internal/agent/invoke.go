package agent

import (
	"context"
	"strings"

	"github.com/cexll/agentsdk-go/pkg/api"
	"github.com/cexll/agentsdk-go/pkg/model"
)

// RunResponse executes a non-streaming turn with the same ContentBlocks + prompt merge rules as the gateway pipeline.
func RunResponse(ctx context.Context, rt Runtime, prompt, sessionID string, contentBlocks []model.ContentBlock) (*api.Response, error) {
	blocks := contentBlocks
	if len(contentBlocks) > 0 && strings.TrimSpace(prompt) != "" {
		blocks = make([]model.ContentBlock, 0, len(contentBlocks)+1)
		blocks = append(blocks, model.ContentBlock{Type: model.ContentBlockText, Text: prompt})
		blocks = append(blocks, contentBlocks...)
		prompt = ""
	}
	return rt.Run(ctx, api.Request{
		Prompt:        prompt,
		ContentBlocks: blocks,
		SessionID:     sessionID,
	})
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
	blocks := contentBlocks
	if len(contentBlocks) > 0 && strings.TrimSpace(prompt) != "" {
		blocks = make([]model.ContentBlock, 0, len(contentBlocks)+1)
		blocks = append(blocks, model.ContentBlock{Type: model.ContentBlockText, Text: prompt})
		blocks = append(blocks, contentBlocks...)
		prompt = ""
	}
	return rt.RunStream(ctx, api.Request{
		Prompt:        prompt,
		ContentBlocks: blocks,
		SessionID:     sessionID,
	})
}
