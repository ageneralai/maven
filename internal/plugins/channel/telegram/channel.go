package telegram

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ageneralai/maven/internal/kernel/httpc"

	"log/slog"

	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/channel"
	"github.com/ageneralai/maven/internal/kernel/channel/allowlist"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const telegramChannelName = "telegram"

type TelegramChannel struct {
	name           string
	log            *slog.Logger
	bus            *bus.MessageBus
	allow          allowlist.Matcher
	token          string
	bot        *telego.Bot
	proxy      string
	httpClient *http.Client
	cancel     context.CancelFunc
	runCtx     context.Context
	feedback   string // "debug", "normal", "minimal", "silent"
	streaming  bool
	workspace  string // workspace root for file saving
	rootDir    string // telegram assets root, default <workspace>/.telegram
	caps           channels.CapabilitySet

	mgMu     sync.Mutex
	mgBuffer map[string]*mediaGroup

	slashCommands map[string]Command
	pipelineSlashes map[string]string
}

func NewTelegramChannel(cfg config.TelegramConfig, workspace string, lg *slog.Logger, b *bus.MessageBus) (*TelegramChannel, error) {
	feedback := cfg.Feedback
	if feedback == "" {
		feedback = "normal"
	}
	tc := &TelegramChannel{
		name:      telegramChannelName,
		log:       lg,
		bus:       b,
		allow:     allowlist.NewMatcher(cfg.AllowFrom),
		token:     cfg.Token,
		proxy:       cfg.Proxy,
		httpClient:  http.DefaultClient,
		feedback:    feedback,
		streaming:   cfg.Streaming,
		rootDir:     strings.TrimSpace(cfg.RootDir),
		workspace:   strings.TrimSpace(workspace),
		runCtx:      context.Background(),
		caps: channels.CapabilitySet{
			Reactions:  true,
			FileUpload: true,
		},
	}
	return tc, nil
}

func (t *TelegramChannel) Name() string {
	return t.name
}

func (t *TelegramChannel) IsAllowed(senderID string) bool {
	return t.allow.Allow(senderID)
}

func (t *TelegramChannel) Capabilities() channels.CapabilitySet { return t.caps }

// PreProcessInbound implements InboundPreprocessor for gateway/pipeline use.
func (t *TelegramChannel) PreProcessInbound(ctx context.Context, chatID int64, hints bus.RoutingHints) {
	_ = ctx
	t.PreProcessFeedback(chatID, hints.MessageID)
}

func (t *TelegramChannel) telegramRoot() string {
	if root := strings.TrimSpace(t.rootDir); root != "" {
		return root
	}
	if strings.TrimSpace(t.workspace) == "" {
		return ""
	}
	return filepath.Join(t.workspace, ".telegram")
}

func (t *TelegramChannel) initBot(ctx context.Context) error {
	var opts []telego.BotOption
	client, err := httpc.ClientFromProxy(t.proxy)
	if err != nil {
		return fmt.Errorf("telegram proxy: %w", err)
	}
	t.httpClient = client
	opts = append(opts, telego.WithHTTPClient(client))

	bot, err := telego.NewBot(t.token, opts...)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}
	t.bot = bot

	me, err := bot.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("telegram getMe: %w", err)
	}
	t.log.Info("telegram authorized", "username", me.Username)
	return nil
}

func (t *TelegramChannel) Start(ctx context.Context) error {
	if err := t.initBot(ctx); err != nil {
		return err
	}
	t.loadSlashCommands()
	if err := t.syncBotCommands(ctx); err != nil {
		return fmt.Errorf("register telegram bot commands: %w", err)
	}

	ctx, t.cancel = context.WithCancel(ctx)
	t.runCtx = ctx

	updates, err := t.bot.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{Timeout: 30})
	if err != nil {
		return fmt.Errorf("start long polling: %w", err)
	}

	go func() {
		for update := range updates {
			if update.Message != nil {
				t.handleMessage(update.Message)
			}
		}
	}()

	t.log.Info("telegram polling started")
	return nil
}

func (t *TelegramChannel) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	t.log.Info("telegram stopped")
	return nil
}

func (t *TelegramChannel) syncBotCommands(ctx context.Context) error {
	if t.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}

	commands := t.registeredBotCommands()
	params := &telego.SetMyCommandsParams{Commands: commands}

	if err := t.bot.SetMyCommands(ctx, params); err != nil {
		return err
	}
	if err := t.bot.SetMyCommands(ctx, (&telego.SetMyCommandsParams{Commands: commands}).WithScope(tu.ScopeAllPrivateChats())); err != nil {
		return err
	}

	t.log.Info("telegram bot commands registered", "count", len(commands))
	return nil
}

var (
	_ channels.Channel                 = (*TelegramChannel)(nil)
	_ channels.StreamChannel           = (*TelegramChannel)(nil)
	_ channels.InboundPreprocessor     = (*TelegramChannel)(nil)
	_ channels.PipelineSlashConfigurer = (*TelegramChannel)(nil)
)
