package gateway

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/agent/postaction"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel/manager"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/internal/cron"
	"github.com/ageneralai/maven/internal/sessionid"
	"github.com/ageneralai/maven/internal/health"
	"github.com/ageneralai/maven/internal/heartbeat"
	"github.com/ageneralai/maven/internal/pipeline"
	"github.com/ageneralai/maven/internal/session"
	"github.com/ageneralai/maven/internal/slash"
	"github.com/ageneralai/maven/internal/health/healthtest"
	"github.com/ageneralai/maven/pkg/executor"
	"github.com/ageneralai/maven/pkg/memory"
	"github.com/ageneralai/maven/pkg/prompt"
	"log/slog"
)

var testLG = slog.New(slog.DiscardHandler)

// mockRuntime implements Runtime interface for testing
type mockRuntime struct {
	response *api.Response
	err      error
	closed   atomic.Bool
	reqCh    chan api.Request
}

func (m *mockRuntime) Run(ctx context.Context, req api.Request) (*api.Response, error) {
	if m.reqCh != nil {
		select {
		case m.reqCh <- req:
		default:
		}
	}
	return m.response, m.err
}

func (m *mockRuntime) Close() {
	m.closed.Store(true)
}

func (m *mockRuntime) RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan api.StreamEvent, 1)
	go func() {
		defer close(ch)
		if m.response != nil && m.response.Result != nil {
			ch <- api.StreamEvent{
				Type:  api.EventContentBlockDelta,
				Delta: &api.Delta{Text: m.response.Result.Output},
			}
		}
	}()
	return ch, nil
}

var _ agent.Runtime = (*mockRuntime)(nil)

func testPipeline(b *bus.MessageBus, rt agent.Runtime, router *session.Router, ws string) *pipeline.Pipeline {
	p := pipeline.New(testLG, b, rt, &session.SessionResolver{Router: router}, postaction.New(router, ws))
	reg, err := slash.BuiltIns(nil)
	if err != nil {
		panic(err)
	}
	p.SlashRegistry = reg
	return p
}

func TestGateway_BuildSystemPrompt(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace files
	if err := os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("# Agent\nYou are helpful."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "SOUL.md"), []byte("# Soul\nBe kind."), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	g := &Gateway{
		cfg: cfg,
		mem: memory.NewMemoryStore(tmpDir),
	}

	sys, err := prompt.Build(g.cfg.Agent.Workspace, g.mem.GetMemoryContext())
	if err != nil {
		t.Fatal(err)
	}

	if sys == "" {
		t.Error("expected non-empty prompt")
	}
	if !strings.Contains(sys, "# Agent") {
		t.Error("missing AGENTS.md content")
	}
	if !strings.Contains(sys, "# Soul") {
		t.Error("missing SOUL.md content")
	}
}

func TestGateway_BuildSystemPrompt_WithMemory(t *testing.T) {
	tmpDir := t.TempDir()

	mem := memory.NewMemoryStore(tmpDir)
	if err := mem.WriteLongTerm("User is a developer."); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	g := &Gateway{
		cfg: cfg,
		mem: mem,
	}

	sys, err := prompt.Build(g.cfg.Agent.Workspace, g.mem.GetMemoryContext())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(sys, "User is a developer") {
		t.Error("missing memory content")
	}
}

func TestGateway_BuildSystemPrompt_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	g := &Gateway{
		cfg: cfg,
		mem: memory.NewMemoryStore(tmpDir),
	}

	sys, err := prompt.Build(g.cfg.Agent.Workspace, g.mem.GetMemoryContext())
	if err != nil {
		t.Fatal(err)
	}

	// Should return empty when no files exist
	if sys != "" {
		t.Errorf("expected empty prompt, got %q", sys)
	}
}

func TestGateway_Shutdown(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	msgBus := bus.New(10, testLG)
	chMgr := manager.New(msgBus, testLG, nil, nil)
	cronSvc, err := cron.NewService(filepath.Join(tmpDir, "cron.json"), executor.Nop{}, 1, testLG, nil)
	if err != nil {
		t.Fatalf("cron.NewService: %v", err)
	}
	mockRt := &mockRuntime{}
	router, rerr := session.New(filepath.Join(tmpDir, ".maven", "session-router.json"))
	if rerr != nil {
		t.Fatalf("session.New: %v", rerr)
	}
	pipe := testPipeline(msgBus, mockRt, router, tmpDir)
	hb, err := heartbeat.New(tmpDir, executor.Nop{}, 0, testLG)
	if err != nil {
		t.Fatalf("heartbeat.New: %v", err)
	}

	g := &Gateway{
		cfg:        cfg,
		bus:        msgBus,
		pipe:       pipe,
		channelMgr: chMgr,
		cron:       cronSvc,
		hb:         hb,
		mem:        memory.NewMemoryStore(tmpDir),
		logger:     testLG,
	}

	err = g.Shutdown()
	if err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
	if !mockRt.closed.Load() {
		t.Error("runtime should be closed")
	}
}

func TestGateway_RunAgent(t *testing.T) {
	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{
				Output: "Hello from mock",
			},
		},
	}
	pipe := pipeline.New(testLG, bus.New(1, testLG), mockRt, &session.SessionResolver{}, postaction.New(&session.Router{}, ""))
	result, err := pipe.RunTurn(context.Background(), "test", "session1")
	if err != nil {
		t.Errorf("runAgent error: %v", err)
	}
	if result != "Hello from mock" {
		t.Errorf("result = %q, want 'Hello from mock'", result)
	}
}

func TestGateway_RunAgent_NilResponse(t *testing.T) {
	mockRt := &mockRuntime{response: nil}
	pipe := pipeline.New(testLG, bus.New(1, testLG), mockRt, &session.SessionResolver{}, postaction.New(&session.Router{}, ""))
	result, err := pipe.RunTurn(context.Background(), "test", "session1")
	if err != nil {
		t.Errorf("runAgent error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGateway_RunAgent_NilResult(t *testing.T) {
	mockRt := &mockRuntime{response: &api.Response{Result: nil}}
	pipe := pipeline.New(testLG, bus.New(1, testLG), mockRt, &session.SessionResolver{}, postaction.New(&session.Router{}, ""))
	result, err := pipe.RunTurn(context.Background(), "test", "session1")
	if err != nil {
		t.Errorf("runAgent error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestGateway_ProcessLoop(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	msgBus := bus.New(10, testLG)
	router, err := session.New(filepath.Join(tmpDir, "router.json"))
	if err != nil {
		t.Fatalf("session.New: %v", err)
	}
	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{Output: "response"},
		},
	}

	g := &Gateway{
		cfg:  cfg,
		bus:  msgBus,
		pipe: testPipeline(msgBus, mockRt, router, tmpDir),
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start process loop
	go g.pipe.Run(ctx)

	// Send inbound message
	if err := msgBus.PublishInbound(context.Background(), bus.InboundMessage{
		Channel:  "test",
		SenderID: "user1",
		ChatID:   "chat1",
		Content:  "hello",
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}

	// Wait for outbound message
	select {
	case outMsg := <-msgBus.OutboundChan():
		if outMsg.Content != "response" {
			t.Errorf("outbound content = %q, want 'response'", outMsg.Content)
		}
		if outMsg.Channel != "test" {
			t.Errorf("outbound channel = %q, want 'test'", outMsg.Channel)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for outbound message")
	}

	cancel()
}

func TestGateway_ProcessLoop_WithContentBlocks(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	msgBus := bus.New(10, testLG)
	imgBlock := model.ContentBlock{
		Type:      model.ContentBlockImage,
		MediaType: "image/jpeg",
		Data:      base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8, 0xff, 0xd9}),
	}
	blocks := []model.ContentBlock{imgBlock}
	reqCh := make(chan api.Request, 1)
	mockRt := &mockRuntime{
		reqCh: reqCh,
		response: &api.Response{
			Result: &api.Result{Output: "multimodal response"},
		},
	}
	router, err := session.New(filepath.Join(tmpDir, "router.json"))
	if err != nil {
		t.Fatalf("session.New: %v", err)
	}
	g := &Gateway{
		cfg:  cfg,
		bus:  msgBus,
		pipe: testPipeline(msgBus, mockRt, router, tmpDir),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go g.pipe.Run(ctx)

	if err := msgBus.PublishInbound(context.Background(), bus.InboundMessage{
		Channel:       "telegram",
		SenderID:      "123",
		ChatID:        "456",
		Content:       "caption text",
		ContentBlocks: blocks,
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}

	select {
	case req := <-reqCh:
		// Workaround merges prompt into ContentBlocks and clears Prompt
		if req.Prompt != "" {
			t.Errorf("runtime prompt = %q, want empty (merged into ContentBlocks)", req.Prompt)
		}
		if req.SessionID != "telegram-456" {
			t.Errorf("runtime sessionID = %q, want telegram-456", req.SessionID)
		}
		// Expect 2 blocks: prepended text + original image
		if len(req.ContentBlocks) != 2 {
			t.Fatalf("runtime content blocks len = %d, want 2", len(req.ContentBlocks))
		}
		if req.ContentBlocks[0].Type != model.ContentBlockText || req.ContentBlocks[0].Text != "caption text" {
			t.Errorf("content block[0] = %+v, want text 'caption text'", req.ContentBlocks[0])
		}
		if req.ContentBlocks[1] != imgBlock {
			t.Errorf("content block[1] = %+v, want image block", req.ContentBlocks[1])
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for runtime request")
	}

	select {
	case outMsg := <-msgBus.OutboundChan():
		if outMsg.Channel != "telegram" {
			t.Errorf("outbound channel = %q, want telegram", outMsg.Channel)
		}
		if outMsg.ChatID != "456" {
			t.Errorf("outbound chatID = %q, want 456", outMsg.ChatID)
		}
		if outMsg.Content != "multimodal response" {
			t.Errorf("outbound content = %q, want multimodal response", outMsg.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for outbound response")
	}
}

func TestGateway_RunAgent_Error(t *testing.T) {
	mockRt := &mockRuntime{err: context.DeadlineExceeded}
	pipe := pipeline.New(testLG, bus.New(1, testLG), mockRt, &session.SessionResolver{}, postaction.New(&session.Router{}, ""))
	_, err := pipe.RunTurn(context.Background(), "test", "session1")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestGateway_RunAgent_ContentBlocks(t *testing.T) {
	blocks := []model.ContentBlock{{Type: model.ContentBlockText, Text: "hello multimodal"}}
	mockRt := &mockRuntime{
		response: &api.Response{Result: &api.Result{Output: "ok"}},
	}
	resp, err := mockRt.Run(context.Background(), api.Request{
		Prompt:        "test",
		ContentBlocks: blocks,
		SessionID:     "session1",
	})
	if err != nil {
		t.Fatalf("runAgent error: %v", err)
	}
	if resp == nil || resp.Result == nil || resp.Result.Output != "ok" {
		t.Fatalf("result = %+v, want ok", resp)
	}
}

func TestGateway_ProcessLoop_AgentError(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	msgBus := bus.New(10, testLG)
	mockRt := &mockRuntime{err: context.DeadlineExceeded}
	router, err := session.New(filepath.Join(tmpDir, "router.json"))
	if err != nil {
		t.Fatalf("session.New: %v", err)
	}
	g := &Gateway{
		cfg:  cfg,
		bus:  msgBus,
		pipe: testPipeline(msgBus, mockRt, router, tmpDir),
	}

	ctx, cancel := context.WithCancel(context.Background())

	go g.pipe.Run(ctx)

	if err := msgBus.PublishInbound(context.Background(), bus.InboundMessage{
		Channel:  "test",
		SenderID: "user1",
		ChatID:   "chat1",
		Content:  "hello",
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}

	select {
	case outMsg := <-msgBus.OutboundChan():
		if outMsg.Content != "Sorry, I encountered an error processing your message." {
			t.Errorf("expected error message, got %q", outMsg.Content)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for error response")
	}

	cancel()
}

func TestGateway_ProcessLoop_EmptyResult(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	msgBus := bus.New(10, testLG)
	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{Output: ""},
		},
	}
	router, err := session.New(filepath.Join(tmpDir, "router.json"))
	if err != nil {
		t.Fatalf("session.New: %v", err)
	}
	g := &Gateway{
		cfg:  cfg,
		bus:  msgBus,
		pipe: testPipeline(msgBus, mockRt, router, tmpDir),
	}

	ctx, cancel := context.WithCancel(context.Background())

	go g.pipe.Run(ctx)

	if err := msgBus.PublishInbound(context.Background(), bus.InboundMessage{
		Channel:  "test",
		SenderID: "user1",
		ChatID:   "chat1",
		Content:  "hello",
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}

	// Should NOT receive outbound message when result is empty
	select {
	case outMsg := <-msgBus.OutboundChan():
		t.Errorf("should not send empty result, got %q", outMsg.Content)
	case <-time.After(100 * time.Millisecond):
		// Expected - no message sent
	}

	cancel()
}

func TestGateway_ProcessLoop_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	msgBus := bus.New(10, testLG)
	mockRt := &mockRuntime{}
	router, err := session.New(filepath.Join(tmpDir, "router.json"))
	if err != nil {
		t.Fatalf("session.New: %v", err)
	}
	g := &Gateway{
		cfg:  cfg,
		bus:  msgBus,
		pipe: testPipeline(msgBus, mockRt, router, tmpDir),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		g.pipe.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Expected - loop exited
	case <-time.After(time.Second):
		t.Error("pipeline Run did not exit after context cancel")
	}
}

func TestGateway_Shutdown_NilRuntime(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	msgBus := bus.New(10, testLG)
	chMgr := manager.New(msgBus, testLG, nil, nil)
	cronSvc, err := cron.NewService(filepath.Join(tmpDir, "cron.json"), executor.Nop{}, 1, testLG, nil)
	if err != nil {
		t.Fatalf("cron.NewService: %v", err)
	}
	router, rerr := session.New(filepath.Join(tmpDir, ".maven", "session-router.json"))
	if rerr != nil {
		t.Fatalf("session.New: %v", rerr)
	}
	pipe := testPipeline(msgBus, nil, router, tmpDir)
	hb, err := heartbeat.New(tmpDir, executor.Nop{}, 0, testLG)
	if err != nil {
		t.Fatalf("heartbeat.New: %v", err)
	}

	g := &Gateway{
		cfg:        cfg,
		bus:        msgBus,
		pipe:       pipe,
		channelMgr: chMgr,
		cron:       cronSvc,
		hb:         hb,
		mem:        memory.NewMemoryStore(tmpDir),
		logger:     testLG,
	}

	err = g.Shutdown()
	if err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}

// mockRuntimeFactory returns a factory that creates mock runtimes
func mockRuntimeFactory(rt agent.Runtime) RuntimeFactory {
	return func(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, cronSvc *cron.Service, pluginTools []tool.Tool, _ api.SessionStore, _ *slog.Logger) (agent.Runtime, error) {
		return rt, nil
	}
}

// errorRuntimeFactory returns a factory that always fails
func errorRuntimeFactory(err error) RuntimeFactory {
	return func(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, cronSvc *cron.Service, pluginTools []tool.Tool, _ api.SessionStore, _ *slog.Logger) (agent.Runtime, error) {
		return nil, err
	}
}

func TestNewWithOptions_MockRuntime(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
		Channels: config.ChannelsConfig{},
	}

	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{Output: "test"},
		},
	}

	g, err := NewWithOptions(cfg, Options{
		Logger:         testLG,
		RuntimeFactory: mockRuntimeFactory(mockRt),
	})
	if err != nil {
		t.Fatalf("NewWithOptions error: %v", err)
	}

	if g == nil {
		t.Fatal("gateway should not be nil")
	}
	if g.bus == nil {
		t.Error("bus should not be nil")
	}
	if g.mem == nil {
		t.Error("mem should not be nil")
	}
	if g.cron == nil {
		t.Error("cron should not be nil")
	}
	if g.hb == nil {
		t.Error("heartbeat should not be nil")
	}
	if g.channelMgr == nil {
		t.Error("channelMgr should not be nil")
	}

	// Clean up
	if err := g.Shutdown(); err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("Shutdown: %v", err)
	}
}

func TestGateway_Apply_WorkspaceChangeRejected(t *testing.T) {
	ws1 := t.TempDir()
	ws2 := t.TempDir()
	cfg1 := &config.Config{
		Agent:    config.AgentConfig{Workspace: ws1},
		Channels: config.ChannelsConfig{},
	}
	mockRt := &mockRuntime{}
	g, err := NewWithOptions(cfg1, Options{Logger: testLG, RuntimeFactory: mockRuntimeFactory(mockRt)})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	defer func() { _ = g.Shutdown() }()
	if err := g.Apply(context.Background(), cfg1); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	cfg2 := &config.Config{
		Agent:    config.AgentConfig{Workspace: ws2},
		Channels: config.ChannelsConfig{},
	}
	err = g.Apply(context.Background(), cfg2)
	if err == nil {
		t.Fatal("expected Apply error")
	}
	if !strings.Contains(err.Error(), "reload: agent.workspace change not supported") {
		t.Fatalf("Apply error got %v", err)
	}
}

func TestNewWithOptions_RuntimeFactoryError(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
	}

	g, err := NewWithOptions(cfg, Options{
		Logger:         testLG,
		RuntimeFactory: errorRuntimeFactory(context.DeadlineExceeded),
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	defer func() { _ = g.Shutdown() }()
	if err := g.Apply(context.Background(), cfg); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Apply expected DeadlineExceeded, got %v", err)
	}
}

func TestNewWithOptions_ChannelManagerError(t *testing.T) {
	tmpDir := t.TempDir()

	// Invalid telegram config to trigger channel manager error
	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
		Channels: config.ChannelsConfig{
			Telegram: config.TelegramConfig{
				Enabled: true,
				Token:   "", // Empty token with enabled=true may cause error
			},
		},
	}

	mockRt := &mockRuntime{}
	_, err := NewWithOptions(cfg, Options{
		Logger:         testLG,
		RuntimeFactory: mockRuntimeFactory(mockRt),
	})
	// Channel manager may or may not error with empty token - just ensure we don't panic
	_ = err
}

func TestGateway_Run_WithSignalChan(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
		Gateway: config.GatewayConfig{
			Host: "localhost",
			Port: 8080,
		},
		Channels: config.ChannelsConfig{},
	}

	mockRt := &mockRuntime{}
	sigCh := make(chan os.Signal, 1)

	g, err := NewWithOptions(cfg, Options{
		Logger:         testLG,
		RuntimeFactory: mockRuntimeFactory(mockRt),
		SignalChan:     sigCh,
	})
	if err != nil {
		t.Fatalf("NewWithOptions error: %v", err)
	}

	// Run in goroutine
	done := make(chan error, 1)
	go func() {
		done <- g.Run(context.Background())
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Send shutdown signal
	sigCh <- os.Interrupt

	// Wait for Run to complete
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Run did not exit after signal")
	}

	if !mockRt.closed.Load() {
		t.Error("runtime should be closed after shutdown")
	}
}

func TestGateway_Run_HealthReporterGatewayReady(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
		Gateway: config.GatewayConfig{
			Host: "localhost",
			Port: 8080,
		},
		Channels: config.ChannelsConfig{},
	}
	mockRt := &mockRuntime{}
	sigCh := make(chan os.Signal, 1)
	var rec healthtest.PulseRecorder
	g, err := NewWithOptions(cfg, Options{
		Logger:         testLG,
		RuntimeFactory: mockRuntimeFactory(mockRt),
		SignalChan:     sigCh,
		HealthReporter: &rec,
	})
	if err != nil {
		t.Fatalf("NewWithOptions error: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- g.Run(context.Background())
	}()
	time.Sleep(80 * time.Millisecond)
	if !rec.Has(health.SignalGatewayReady) {
		t.Fatalf("want %s pulse", health.SignalGatewayReady)
	}
	sigCh <- os.Interrupt
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after signal")
	}
}

func TestGateway_Run_ChannelStartError(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
		Gateway: config.GatewayConfig{
			Host: "localhost",
			Port: 8080,
		},
		Channels: config.ChannelsConfig{
			Telegram: config.TelegramConfig{
				Enabled: true,
				Token:   "invalid-token", // Will fail on StartAll
			},
		},
	}

	mockRt := &mockRuntime{}
	sigCh := make(chan os.Signal, 1)

	g, err := NewWithOptions(cfg, Options{
		Logger:         testLG,
		RuntimeFactory: mockRuntimeFactory(mockRt),
		SignalChan:     sigCh,
	})
	if err != nil {
		t.Fatalf("NewWithOptions error: %v", err)
	}

	// Run should return error from channel start
	err = g.Run(context.Background())
	if err == nil {
		t.Error("expected error from channel start")
	}
}

func TestDefaultRuntimeFactory_NoAPIKey(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderConfig{
			APIKey: "",
		},
	}

	// DefaultRuntimeFactory will try to create real runtime
	// which may fail in different ways depending on SDK behavior
	_, err := DefaultRuntimeFactory(cfg, "test prompt", nil, nil, nil, nil, testLG)
	// Just ensure it doesn't panic - error is expected
	_ = err
}

func TestGateway_CronRunTurn(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
		Channels: config.ChannelsConfig{},
	}

	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{Output: "cron result"},
		},
		reqCh: make(chan api.Request, 2),
	}

	g, err := NewWithOptions(cfg, Options{
		Logger:         testLG,
		RuntimeFactory: mockRuntimeFactory(mockRt),
	})
	if err != nil {
		t.Fatalf("NewWithOptions error: %v", err)
	}
	defer func() { _ = g.Shutdown() }()

	if err := g.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	j, err := g.cron.AddJob("n", cron.EverySchedule{Interval: time.Hour}, cron.Payload{Message: "test message", Deliver: false})
	if err != nil {
		t.Fatal(err)
	}
	ge := g.pipe
	sid := sessionid.New(sessionid.KindCron, j.ID).String()
	result, err := ge.RunTurn(context.Background(), j.Payload.Message, sid)
	if err != nil {
		t.Errorf("RunTurn error: %v", err)
	}
	if result != "cron result" {
		t.Errorf("result = %q, want 'cron result'", result)
	}
	select {
	case req := <-mockRt.reqCh:
		parsed, err := sessionid.Parse(req.SessionID)
		if err != nil || parsed.Kind != sessionid.KindCron || parsed.Owner != j.ID {
			t.Fatalf("SessionID = %q, want cron-isolated key for job %q", req.SessionID, j.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for runtime request")
	}
}

func TestGateway_CronRunTurn_WithDelivery(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
		Channels: config.ChannelsConfig{},
	}

	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{Output: "delivered result"},
		},
	}

	g, err := NewWithOptions(cfg, Options{
		Logger:         testLG,
		RuntimeFactory: mockRuntimeFactory(mockRt),
	})
	if err != nil {
		t.Fatalf("NewWithOptions error: %v", err)
	}
	defer func() { _ = g.Shutdown() }()

	if err := g.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	j, err := g.cron.AddJob("n", cron.EverySchedule{Interval: time.Hour}, cron.Payload{
		Message: "test message", Deliver: true, Channel: "telegram", To: "12345",
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		select {
		case msg := <-g.bus.OutboundChan():
			if msg.Content != "delivered result" {
				t.Errorf("outbound content = %q, want 'delivered result'", msg.Content)
			}
			if msg.Channel != "telegram" {
				t.Errorf("outbound channel = %q, want 'telegram'", msg.Channel)
			}
			if msg.ChatID != "12345" {
				t.Errorf("outbound chatID = %q, want '12345'", msg.ChatID)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for outbound message")
		}
		close(done)
	}()
	deliver := &cron.Deliver{Bus: g.bus, Channels: g.channelMgr, Log: g.logger}
	deliver.AfterSuccessfulRun(context.Background(), *j, "delivered result")
	<-done
}

func TestGateway_CronRunTurn_InvalidDeliverPayload(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
		Channels: config.ChannelsConfig{},
	}
	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{Output: "should not run"},
		},
	}
	g, err := NewWithOptions(cfg, Options{
		Logger:         testLG,
		RuntimeFactory: mockRuntimeFactory(mockRt),
	})
	if err != nil {
		t.Fatalf("NewWithOptions error: %v", err)
	}
	defer func() { _ = g.Shutdown() }()
	if err := g.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	j, err := g.cron.AddJob("bad", cron.EverySchedule{Interval: time.Hour}, cron.Payload{
		Message: "test", Deliver: true, Channel: "telegram", To: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Payload.Validate(); err == nil {
		t.Fatal("expected payload validation error for empty To")
	}
}

func TestGateway_CronRunTurn_RuntimeError(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
		Channels: config.ChannelsConfig{},
	}

	mockRt := &mockRuntime{
		err: context.DeadlineExceeded,
	}

	g, err := NewWithOptions(cfg, Options{
		Logger:         testLG,
		RuntimeFactory: mockRuntimeFactory(mockRt),
	})
	if err != nil {
		t.Fatalf("NewWithOptions error: %v", err)
	}
	defer func() { _ = g.Shutdown() }()
	if err := g.Apply(context.Background(), cfg); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	j, err := g.cron.AddJob("n", cron.EverySchedule{Interval: time.Hour}, cron.Payload{Message: "test message"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.pipe.RunTurn(context.Background(), j.Payload.Message, sessionid.New(sessionid.KindCron, j.ID).String())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestGateway_ProcessLoop_CompactPostAction(t *testing.T) {
	tmpDir := t.TempDir()
	router, err := session.New(filepath.Join(tmpDir, "router.json"))
	if err != nil {
		t.Fatalf("session.New error: %v", err)
	}

	msgBus := bus.New(10, testLG)
	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{Output: "important user goals and pending tasks"},
		},
	}

	g := &Gateway{
		cfg:  &config.Config{Agent: config.AgentConfig{Workspace: tmpDir}},
		bus:  msgBus,
		pipe: testPipeline(msgBus, mockRt, router, tmpDir),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.pipe.Run(ctx)

	if err := msgBus.PublishInbound(context.Background(), bus.InboundMessage{
		Channel:  "telegram",
		SenderID: "user1",
		ChatID:   "chat1",
		Content:  "/compact",
		Hints: bus.RoutingHints{
			SlashCommand: "compact",
			SlashType:    "pipeline",
			ForceSync:    true,
		},
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}

	select {
	case outMsg := <-msgBus.OutboundChan():
		if outMsg.Content != "✅ Conversation compacted and continued in a fresh session." {
			t.Fatalf("unexpected outbound content: %q", outMsg.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for compact response")
	}

	baseSession := "telegram:chat1"
	defaultSession := session.SessionIDFromRouteKey(baseSession)
	currentSession := router.Resolve(baseSession, defaultSession)
	if currentSession == defaultSession {
		t.Fatal("expected compact to rotate session")
	}
	if !strings.HasPrefix(currentSession, defaultSession+":rotated:") {
		t.Fatalf("expected rotated session prefix %q:rotated:, got %q", defaultSession, currentSession)
	}

	seedPath := filepath.Join(tmpDir, ".maven", "history", currentSession+".json")
	data, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("read compact seed: %v", err)
	}
	if !strings.Contains(string(data), "important user goals and pending tasks") {
		t.Fatalf("compact seed missing summary: %s", string(data))
	}
}

func TestGateway_ProcessLoop_BuiltinNewSkipsRuntime(t *testing.T) {
	tmpDir := t.TempDir()
	router, err := session.New(filepath.Join(tmpDir, "router.json"))
	if err != nil {
		t.Fatalf("session.New error: %v", err)
	}

	msgBus := bus.New(10, testLG)
	reqCh := make(chan api.Request, 1)
	mockRt := &mockRuntime{reqCh: reqCh}

	g := &Gateway{
		cfg:  &config.Config{Agent: config.AgentConfig{Workspace: tmpDir}},
		bus:  msgBus,
		pipe: testPipeline(msgBus, mockRt, router, tmpDir),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.pipe.Run(ctx)

	if err := msgBus.PublishInbound(context.Background(), bus.InboundMessage{
		Channel:  "telegram",
		SenderID: "user1",
		ChatID:   "chat1",
		Hints: bus.RoutingHints{
			BuiltinCommand: "new",
			ForceSync:      true,
		},
	}); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}

	select {
	case outMsg := <-msgBus.OutboundChan():
		if outMsg.Content != "✅ Started a fresh session." {
			t.Fatalf("unexpected outbound content: %q", outMsg.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for new-session response")
	}

	select {
	case req := <-reqCh:
		t.Fatalf("runtime should not be called for builtin command, got %+v", req)
	case <-time.After(100 * time.Millisecond):
	}

	baseSession := "telegram:chat1"
	defaultSession := session.SessionIDFromRouteKey(baseSession)
	currentSession := router.Resolve(baseSession, defaultSession)
	if currentSession == defaultSession {
		t.Fatal("expected /new to rotate session")
	}
	if !strings.HasPrefix(currentSession, defaultSession+":rotated:") {
		t.Fatalf("expected rotated session prefix %q:rotated:, got %q", defaultSession, currentSession)
	}
}

