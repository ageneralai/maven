package gateway

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/internal/cron"
	"github.com/ageneralai/maven/internal/cronschedule"
	"github.com/ageneralai/maven/internal/heartbeat"
	"github.com/ageneralai/maven/internal/inboundctx"
	"github.com/ageneralai/maven/internal/memory"
	"github.com/ageneralai/maven/internal/runtimecmd"
	"github.com/ageneralai/maven/internal/session"
	"github.com/ageneralai/maven/internal/skills"
	"github.com/cexll/agentsdk-go/pkg/api"
	"github.com/cexll/agentsdk-go/pkg/model"
	"github.com/cexll/agentsdk-go/pkg/tool"
)

// automationSessionID is the only session used for cron and heartbeat (single automation lane).
const automationSessionID = "system"

// heartbeatSkipReasonAutomationLaneBusy is logged when heartbeat yields because cron holds agentMu.
const heartbeatSkipReasonAutomationLaneBusy = "automation_lane_busy"

// Runtime interface for agent runtime (allows mocking in tests)
type Runtime interface {
	Run(ctx context.Context, req api.Request) (*api.Response, error)
	RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error)
	Close()
}

// runtimeAdapter wraps api.Runtime to implement Runtime interface
type runtimeAdapter struct {
	rt *api.Runtime
}

func (r *runtimeAdapter) Run(ctx context.Context, req api.Request) (*api.Response, error) {
	return r.rt.Run(ctx, req)
}

func (r *runtimeAdapter) RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	return r.rt.RunStream(ctx, req)
}

func (r *runtimeAdapter) Close() {
	r.rt.Close()
}

// RuntimeFactory creates a Runtime instance
type RuntimeFactory func(cfg *config.Config, sysPrompt string) (Runtime, error)

// Options for creating a Gateway
type Options struct {
	RuntimeFactory RuntimeFactory
	SignalChan     chan os.Signal // for testing signal handling
}

// DefaultRuntimeFactory creates the default agentsdk-go runtime
func DefaultRuntimeFactory(cfg *config.Config, sysPrompt string) (Runtime, error) {
	return newRuntime(cfg, sysPrompt, nil, nil)
}

func newRuntime(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, cronSvc *cron.Service) (Runtime, error) {
	var provider api.ModelFactory
	switch cfg.Provider.Type {
	case "openai":
		provider = &model.OpenAIProvider{
			APIKey:    cfg.Provider.APIKey,
			BaseURL:   cfg.Provider.BaseURL,
			ModelName: cfg.Agent.Model,
			MaxTokens: cfg.Agent.MaxTokens,
		}
	default:
		provider = &model.AnthropicProvider{
			APIKey:    cfg.Provider.APIKey,
			BaseURL:   cfg.Provider.BaseURL,
			ModelName: cfg.Agent.Model,
			MaxTokens: cfg.Agent.MaxTokens,
		}
	}

	rt, err := api.New(context.Background(), api.Options{
		ProjectRoot:   cfg.Agent.Workspace,
		ModelFactory:  provider,
		SystemPrompt:  sysPrompt,
		MaxIterations: cfg.Agent.MaxToolIterations,
		MCPServers:    cfg.MCP.Servers,
		TokenTracking: cfg.TokenTracking.Enabled,
		AutoCompact: api.CompactConfig{
			Enabled:       cfg.AutoCompact.Enabled,
			Threshold:     cfg.AutoCompact.Threshold,
			PreserveCount: cfg.AutoCompact.PreserveCount,
		},
		Skills:      skillRegs,
		Commands:    runtimecmd.Build(cronSvc),
		CustomTools: customCronTools(cronSvc),
	})
	if err != nil {
		return nil, fmt.Errorf("create runtime: %w", err)
	}
	return &runtimeAdapter{rt: rt}, nil
}

func customCronTools(cronSvc *cron.Service) []tool.Tool {
	return cronschedule.Tools(cronSvc)
}

type Gateway struct {
	cfg        *config.Config
	bus        *bus.MessageBus
	runtime    Runtime
	channels   *channel.ChannelManager
	cron       *cron.Service
	hb         *heartbeat.Service
	agentMu    sync.Mutex
	mem        *memory.MemoryStore
	skillRegs  []api.SkillRegistration
	sessions   *session.Router
	signalChan chan os.Signal // for testing
}

// New creates a Gateway with default options
func New(cfg *config.Config) (*Gateway, error) {
	return NewWithOptions(cfg, Options{})
}

// NewWithOptions creates a Gateway with custom options for testing
func NewWithOptions(cfg *config.Config, opts Options) (*Gateway, error) {
	g := &Gateway{cfg: cfg}

	// Message bus
	g.bus = bus.NewMessageBus(config.DefaultBufSize)

	// Memory
	g.mem = memory.NewMemoryStore(cfg.Agent.Workspace)

	router, routerErr := session.New(filepath.Join(cfg.Agent.Workspace, ".maven", "session-router.json"))
	if routerErr != nil {
		return nil, routerErr
	}
	g.sessions = router

	// Build system prompt
	sysPrompt := g.buildSystemPrompt()

	if cfg.Skills.Enabled {
		skillDir := cfg.Skills.Dir
		if skillDir == "" {
			skillDir = filepath.Join(cfg.Agent.Workspace, "skills")
		}
		skillRegs, err := skills.LoadSkills(skillDir)
		if err != nil {
			log.Printf("[gateway] skills load warning: %v", err)
		}
		g.skillRegs = skillRegs
	}

	cronStorePath := filepath.Join(config.ConfigDir(), "data", "cron", "jobs.json")
	g.cron = cron.NewService(cronStorePath)

	// Create runtime using factory (allows injection for testing)
	factory := opts.RuntimeFactory
	var (
		rt  Runtime
		err error
	)
	if factory == nil {
		rt, err = newRuntime(cfg, sysPrompt, g.skillRegs, g.cron)
	} else {
		rt, err = factory(cfg, sysPrompt)
	}
	if err != nil {
		return nil, err
	}
	g.runtime = rt

	// Signal channel for testing
	g.signalChan = opts.SignalChan

	runAgent := func(prompt string) (string, error) {
		return g.runAgent(context.Background(), prompt, automationSessionID, nil)
	}
	g.cron.OnJob = func(job cron.CronJob) (string, error) {
		if err := job.Payload.Validate(); err != nil {
			return "", err
		}
		g.agentMu.Lock()
		defer g.agentMu.Unlock()
		result, err := runAgent(job.Payload.Message)
		if err != nil {
			return "", err
		}
		if job.Payload.Deliver {
			g.deliverCronOutbound(job.Payload.Channel, job.Payload.To, result)
		}
		return result, nil
	}
	g.hb = heartbeat.New(cfg.Agent.Workspace, g.heartbeatAgentTurn, 0)

	// Channels (with gateway config for WebUI port)
	chMgr, err := channel.NewChannelManagerWithGateway(cfg.Channels, cfg.Gateway, g.bus)
	if err != nil {
		return nil, fmt.Errorf("create channel manager: %w", err)
	}
	g.channels = chMgr

	// Set workspace on Telegram channel for file saving.
	if tc, ok := chMgr.GetChannel("telegram").(*channel.TelegramChannel); ok {
		tc.SetWorkspace(cfg.Agent.Workspace)
	}

	return g, nil
}

func (g *Gateway) deliverCronOutbound(channel, chatID, body string) {
	g.bus.Outbound <- bus.OutboundMessage{
		Channel: channel,
		ChatID:  chatID,
		Content: body,
	}
}

func (g *Gateway) heartbeatAgentTurn(prompt string) (string, error) {
	if !g.agentMu.TryLock() {
		log.Printf("[heartbeat] skipped: %s", heartbeatSkipReasonAutomationLaneBusy)
		return "", nil
	}
	defer g.agentMu.Unlock()
	return g.runAgent(context.Background(), prompt, automationSessionID, nil)
}

func (g *Gateway) buildSystemPrompt() string {
	var sb strings.Builder

	if data, err := os.ReadFile(filepath.Join(g.cfg.Agent.Workspace, "AGENTS.md")); err == nil {
		sb.Write(data)
		sb.WriteString("\n\n")
	}

	if data, err := os.ReadFile(filepath.Join(g.cfg.Agent.Workspace, "SOUL.md")); err == nil {
		sb.Write(data)
		sb.WriteString("\n\n")
	}

	if memCtx := g.mem.GetMemoryContext(); memCtx != "" {
		sb.WriteString(memCtx)
	}

	return sb.String()
}

func (g *Gateway) runAgent(ctx context.Context, prompt, sessionID string, contentBlocks []model.ContentBlock) (string, error) {
	resp, err := g.runAgentResponse(ctx, prompt, sessionID, contentBlocks)
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Result == nil {
		return "", nil
	}
	return resp.Result.Output, nil
}

func (g *Gateway) runAgentResponse(ctx context.Context, prompt, sessionID string, contentBlocks []model.ContentBlock) (*api.Response, error) {
	// Workaround: agentsdk-go drops Prompt when ContentBlocks exist (anthropic.go:420-431).
	// Merge text prompt into ContentBlocks so both text and media reach the API.
	blocks := contentBlocks
	if len(contentBlocks) > 0 && strings.TrimSpace(prompt) != "" {
		blocks = make([]model.ContentBlock, 0, len(contentBlocks)+1)
		blocks = append(blocks, model.ContentBlock{Type: model.ContentBlockText, Text: prompt})
		blocks = append(blocks, contentBlocks...)
		prompt = "" // clear to avoid duplication if SDK is fixed later
	}

	resp, err := g.runtime.Run(ctx, api.Request{
		Prompt:        prompt,
		ContentBlocks: blocks,
		SessionID:     sessionID,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (g *Gateway) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go g.bus.DispatchOutbound(ctx)

	if err := g.channels.StartAll(ctx); err != nil {
		return fmt.Errorf("start channels: %w", err)
	}
	log.Printf("[gateway] channels started: %v", g.channels.EnabledChannels())

	if err := g.cron.Start(ctx); err != nil {
		log.Printf("[gateway] cron start warning: %v", err)
	}

	go func() {
		if err := g.hb.Start(ctx); err != nil {
			log.Printf("[gateway] heartbeat error: %v", err)
		}
	}()

	go g.processLoop(ctx)

	log.Printf("[gateway] running on %s:%d", g.cfg.Gateway.Host, g.cfg.Gateway.Port)

	// Use injected signal channel for testing, or create default
	sigCh := g.signalChan
	if sigCh == nil {
		sigCh = make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	}
	<-sigCh

	log.Printf("[gateway] shutting down...")
	return g.Shutdown()
}

// StreamSender is an optional interface channels can implement for streaming output.
type StreamSender interface {
	SendStream(ctx context.Context, chatID string, metadata map[string]any, events <-chan api.StreamEvent) error
}

func (g *Gateway) processLoop(ctx context.Context) {
	for {
		select {
		case msg := <-g.bus.Inbound:
			log.Printf("[gateway] inbound from %s/%s: %s", msg.Channel, msg.SenderID, truncate(msg.Content, 80))

			if handled, err := g.handleBuiltinCommand(msg); handled {
				if err != nil {
					log.Printf("[gateway] builtin command error: %v", err)
					g.bus.Outbound <- bus.OutboundMessage{
						Channel: msg.Channel,
						ChatID:  msg.ChatID,
						Content: "Sorry, I encountered an error processing your command.",
					}
				} else {
					g.bus.Outbound <- bus.OutboundMessage{
						Channel:  msg.Channel,
						ChatID:   msg.ChatID,
						Content:  "✅ Started a fresh session.",
						Metadata: msg.Metadata,
					}
				}
				continue
			}

			msgCtx := inboundctx.With(ctx, msg.Channel, msg.ChatID)

			sessionKey := msg.SessionKey()
			if shouldIsolateSession(msg.Metadata) {
				sessionKey = isolatedSessionKey(msg)
			} else if g.sessions != nil {
				sessionKey = g.sessions.Resolve(sessionKey, sessionKey)
			}

			var ch channel.Channel
			if g.channels != nil {
				ch = g.channels.GetChannel(msg.Channel)
			}

			// Pre-processing feedback
			if ch != nil {
				if tc, ok := ch.(*channel.TelegramChannel); ok {
					chatIDInt := mustParseChatID(msg.ChatID)
					msgID := extractMessageID(msg.Metadata)
					tc.PreProcessFeedback(chatIDInt, msgID)
				}
			}

			// Check if channel supports streaming
			if ch != nil && !shouldForceNonStreaming(msg.Metadata) {
				if ss, ok := ch.(StreamSender); ok {
					events, err := g.runAgentStream(msgCtx, msg.Content, sessionKey, msg.ContentBlocks)
					if err != nil {
						log.Printf("[gateway] agent stream error: %v", err)
						g.bus.Outbound <- bus.OutboundMessage{
							Channel: msg.Channel,
							ChatID:  msg.ChatID,
							Content: "Sorry, I encountered an error processing your message.",
						}
						continue
					}
					if err := ss.SendStream(ctx, msg.ChatID, msg.Metadata, events); err != nil {
						log.Printf("[gateway] SendStream error: %v", err)
					}
					continue
				}
			}

			// Non-streaming path
			resp, err := g.runAgentResponse(msgCtx, msg.Content, sessionKey, msg.ContentBlocks)
			if err != nil {
				log.Printf("[gateway] agent error: %v", err)
				g.bus.Outbound <- bus.OutboundMessage{
					Channel: msg.Channel,
					ChatID:  msg.ChatID,
					Content: "Sorry, I encountered an error processing your message.",
				}
				continue
			}

			result := ""
			if resp != nil && resp.Result != nil {
				result = resp.Result.Output
			}
			if postResult, handled, postErr := g.handlePostResponse(msg.SessionKey(), resp); handled || postErr != nil {
				if postErr != nil {
					log.Printf("[gateway] post action error: %v", postErr)
					result = "Sorry, I encountered an error processing your command."
				} else {
					result = postResult
				}
			}

			if result != "" {
				g.bus.Outbound <- bus.OutboundMessage{
					Channel:  msg.Channel,
					ChatID:   msg.ChatID,
					Content:  result,
					Metadata: msg.Metadata,
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func shouldForceNonStreaming(meta map[string]any) bool {
	if meta == nil {
		return false
	}
	v, ok := meta["force_non_streaming"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func shouldIsolateSession(meta map[string]any) bool {
	return metadataString(meta, "session_mode") == "isolated"
}

func isolatedSessionKey(msg bus.InboundMessage) string {
	return msg.SessionKey() + "#isolated#" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func (g *Gateway) Shutdown() error {
	g.cron.Stop()
	_ = g.channels.StopAll()
	if g.runtime != nil {
		g.runtime.Close()
	}
	log.Printf("[gateway] shutdown complete")
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// runAgentStream calls RunStream on the runtime and returns the event channel.
func (g *Gateway) runAgentStream(ctx context.Context, prompt, sessionID string, contentBlocks []model.ContentBlock) (<-chan api.StreamEvent, error) {
	blocks := contentBlocks
	if len(contentBlocks) > 0 && strings.TrimSpace(prompt) != "" {
		blocks = make([]model.ContentBlock, 0, len(contentBlocks)+1)
		blocks = append(blocks, model.ContentBlock{Type: model.ContentBlockText, Text: prompt})
		blocks = append(blocks, contentBlocks...)
		prompt = ""
	}
	return g.runtime.RunStream(ctx, api.Request{
		Prompt:        prompt,
		ContentBlocks: blocks,
		SessionID:     sessionID,
	})
}

// mustParseChatID parses a chat ID string, returning 0 on error.
func mustParseChatID(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// extractMessageID extracts message_id from metadata map.
func extractMessageID(meta map[string]any) int {
	if meta == nil {
		return 0
	}
	if v, ok := meta["message_id"]; ok {
		switch id := v.(type) {
		case int:
			return id
		case int64:
			return int(id)
		case float64:
			return int(id)
		}
	}
	return 0
}
