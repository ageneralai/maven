package acp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ageneralai/maven/internal/kernel/config"
	acpsdk "github.com/coder/acp-go-sdk"
)

type delegateClient struct {
	emit      func(string, bool)
	finalMu   sync.Mutex
	final     strings.Builder
	workspace string
	restrict  bool
}

var _ acpsdk.Client = (*delegateClient)(nil)

func (c *delegateClient) RequestPermission(ctx context.Context, params acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	_ = ctx
	for _, o := range params.Options {
		if o.Kind == acpsdk.PermissionOptionKindAllowOnce || o.Kind == acpsdk.PermissionOptionKindAllowAlways {
			return acpsdk.RequestPermissionResponse{Outcome: acpsdk.RequestPermissionOutcome{Selected: &acpsdk.RequestPermissionOutcomeSelected{OptionId: o.OptionId}}}, nil
		}
	}
	if len(params.Options) > 0 {
		return acpsdk.RequestPermissionResponse{Outcome: acpsdk.RequestPermissionOutcome{Selected: &acpsdk.RequestPermissionOutcomeSelected{OptionId: params.Options[0].OptionId}}}, nil
	}
	return acpsdk.RequestPermissionResponse{Outcome: acpsdk.RequestPermissionOutcome{Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{}}}, nil
}

func (c *delegateClient) SessionUpdate(ctx context.Context, params acpsdk.SessionNotification) error {
	_ = ctx
	u := params.Update
	switch {
	case u.AgentThoughtChunk != nil:
		thought := u.AgentThoughtChunk.Content
		if thought.Text != nil && thought.Text.Text != "" {
			c.emit("💭 "+thought.Text.Text+"\n", false)
		}
	case u.ToolCall != nil:
		tc := u.ToolCall
		title := strings.TrimSpace(tc.Title)
		line := fmt.Sprintf("🔧 %s: %s\n", title, tc.Status)
		c.emit(line, false)
		for _, block := range tc.Content {
			if block.Diff != nil && strings.TrimSpace(block.Diff.Path) != "" {
				c.emit("📄 "+block.Diff.Path+"\n", false)
			}
		}
	case u.ToolCallUpdate != nil:
		tu := u.ToolCallUpdate
		title := ""
		if tu.Title != nil {
			title = strings.TrimSpace(*tu.Title)
		}
		st := ""
		if tu.Status != nil {
			st = string(*tu.Status)
		}
		line := fmt.Sprintf("🔧 %s · %s · %s\n", tu.ToolCallId, title, st)
		c.emit(line, false)
		for _, block := range tu.Content {
			if block.Diff != nil && strings.TrimSpace(block.Diff.Path) != "" {
				c.emit("📄 "+block.Diff.Path+"\n", false)
			}
		}
	case u.AgentMessageChunk != nil:
		content := u.AgentMessageChunk.Content
		if content.Text != nil && content.Text.Text != "" {
			c.finalMu.Lock()
			c.final.WriteString(content.Text.Text)
			c.finalMu.Unlock()
		}
	case u.UserMessageChunk != nil, u.Plan != nil:
	}
	return nil
}

func (c *delegateClient) WriteTextFile(ctx context.Context, params acpsdk.WriteTextFileRequest) (acpsdk.WriteTextFileResponse, error) {
	_ = ctx
	if !filepath.IsAbs(params.Path) {
		return acpsdk.WriteTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	if _, err := resolveWorkspacePath(c.workspace, c.restrict, params.Path); err != nil {
		return acpsdk.WriteTextFileResponse{}, err
	}
	dir := filepath.Dir(params.Path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return acpsdk.WriteTextFileResponse{}, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(params.Path, []byte(params.Content), 0o644); err != nil {
		return acpsdk.WriteTextFileResponse{}, fmt.Errorf("write %s: %w", params.Path, err)
	}
	return acpsdk.WriteTextFileResponse{}, nil
}

func (c *delegateClient) ReadTextFile(ctx context.Context, params acpsdk.ReadTextFileRequest) (acpsdk.ReadTextFileResponse, error) {
	_ = ctx
	if !filepath.IsAbs(params.Path) {
		return acpsdk.ReadTextFileResponse{}, fmt.Errorf("path must be absolute: %s", params.Path)
	}
	if _, err := resolveWorkspacePath(c.workspace, c.restrict, params.Path); err != nil {
		return acpsdk.ReadTextFileResponse{}, err
	}
	b, err := os.ReadFile(params.Path)
	if err != nil {
		return acpsdk.ReadTextFileResponse{}, fmt.Errorf("read %s: %w", params.Path, err)
	}
	content := string(b)
	if params.Line != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		start := 0
		if params.Line != nil && *params.Line > 0 {
			start = *params.Line - 1
			if start < 0 {
				start = 0
			}
			if start > len(lines) {
				start = len(lines)
			}
		}
		end := len(lines)
		if params.Limit != nil && *params.Limit > 0 {
			if start+*params.Limit < end {
				end = start + *params.Limit
			}
		}
		content = strings.Join(lines[start:end], "\n")
	}
	return acpsdk.ReadTextFileResponse{Content: content}, nil
}

func (c *delegateClient) CreateTerminal(ctx context.Context, params acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
	_ = ctx
	_ = params
	return acpsdk.CreateTerminalResponse{}, fmt.Errorf("terminal not supported in gateway delegate mode")
}

func (c *delegateClient) TerminalOutput(ctx context.Context, params acpsdk.TerminalOutputRequest) (acpsdk.TerminalOutputResponse, error) {
	_ = ctx
	_ = params
	return acpsdk.TerminalOutputResponse{}, fmt.Errorf("terminal not supported in gateway delegate mode")
}

func (c *delegateClient) ReleaseTerminal(ctx context.Context, params acpsdk.ReleaseTerminalRequest) (acpsdk.ReleaseTerminalResponse, error) {
	_ = ctx
	_ = params
	return acpsdk.ReleaseTerminalResponse{}, fmt.Errorf("terminal not supported in gateway delegate mode")
}

func (c *delegateClient) WaitForTerminalExit(ctx context.Context, params acpsdk.WaitForTerminalExitRequest) (acpsdk.WaitForTerminalExitResponse, error) {
	_ = ctx
	_ = params
	return acpsdk.WaitForTerminalExitResponse{}, fmt.Errorf("terminal not supported in gateway delegate mode")
}

func (c *delegateClient) KillTerminal(ctx context.Context, params acpsdk.KillTerminalRequest) (acpsdk.KillTerminalResponse, error) {
	_ = ctx
	_ = params
	return acpsdk.KillTerminalResponse{}, fmt.Errorf("terminal not supported in gateway delegate mode")
}

// runACPSession spawns a single agent subprocess for one Prompt turn; ctx cancellation kills the process.
func runACPSession(ctx context.Context, agent config.ACPAgent, cwd, userPrompt, workspace string, restrict bool, emit func(string, bool)) (string, error) {
	if emit == nil {
		emit = func(string, bool) {}
	}
	cli := &delegateClient{emit: emit, workspace: workspace, restrict: restrict}
	var stderrBuf strings.Builder
	cmd := exec.CommandContext(ctx, agent.Command, agent.Args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), agent.Env...)
	cmd.Stderr = &stderrBuf
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("acp stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("acp stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", wrapErrWithStderr(fmt.Errorf("start acp agent: %w", err), &stderrBuf)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	conn := acpsdk.NewClientSideConnection(cli, stdinPipe, stdoutPipe)
	conn.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := conn.Initialize(ctx, acpsdk.InitializeRequest{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		ClientCapabilities: acpsdk.ClientCapabilities{
			Fs: acpsdk.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
		},
	}); err != nil {
		return "", wrapErrWithStderr(fmt.Errorf("acp initialize: %w", err), &stderrBuf)
	}
	sess, err := conn.NewSession(ctx, acpsdk.NewSessionRequest{Cwd: cwd, McpServers: []acpsdk.McpServer{}})
	if err != nil {
		return "", wrapErrWithStderr(fmt.Errorf("acp new session: %w", err), &stderrBuf)
	}
	if _, err := conn.Prompt(ctx, acpsdk.PromptRequest{
		SessionId: sess.SessionId,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock(userPrompt)},
	}); err != nil {
		return "", wrapErrWithStderr(fmt.Errorf("acp prompt: %w", err), &stderrBuf)
	}
	cli.finalMu.Lock()
	out := strings.TrimSpace(cli.final.String())
	cli.finalMu.Unlock()
	return out, nil
}

func wrapErrWithStderr(err error, stderr *strings.Builder) error {
	if err == nil {
		return nil
	}
	if stderr == nil || stderr.Len() == 0 {
		return err
	}
	return fmt.Errorf("%w\nacp subprocess stderr:\n%s", err, strings.TrimSpace(stderr.String()))
}
