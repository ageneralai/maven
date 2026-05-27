package telegram

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ageneralai/maven/pkg/httpc"

	"log/slog"

	chann "github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/config"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const telegramChannelName = "telegram"

type TelegramChannel struct {
	chann.BaseChannel
	token      string
	bot        *telego.Bot
	proxy      string
	httpClient *http.Client
	cancel     context.CancelFunc
	runCtx     context.Context
	feedback   string // "debug", "normal", "minimal", "silent"
	streaming  bool
	workspace  string // workspace root for file saving
	rootDir    string // telegram assets root, default <workspace>/.telegram
	caps       chann.CapabilitySet

	mgMu     sync.Mutex
	mgBuffer map[string]*mediaGroup

	slashCommands map[string]Command
}

func NewTelegramChannel(cfg config.TelegramConfig, workspace string, lg *slog.Logger, b *bus.MessageBus) (*TelegramChannel, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram token is required")
	}
	feedback := cfg.Feedback
	if feedback == "" {
		feedback = "normal"
	}
	tc := &TelegramChannel{
		BaseChannel: chann.NewBaseChannel(telegramChannelName, b, cfg.AllowFrom, lg),
		token:       cfg.Token,
		proxy:       cfg.Proxy,
		httpClient:  http.DefaultClient,
		feedback:    feedback,
		streaming:   cfg.Streaming,
		rootDir:     strings.TrimSpace(cfg.RootDir),
		workspace:   strings.TrimSpace(workspace),
		runCtx:      context.Background(),
		caps: chann.CapabilitySet{
			Reactions:  true,
			FileUpload: true,
		},
	}
	return tc, nil
}

func (t *TelegramChannel) Capabilities() chann.CapabilitySet { return t.caps }

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

func (t *TelegramChannel) initBot() error {
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

	me, err := bot.GetMe(context.Background())
	if err != nil {
		return fmt.Errorf("telegram getMe: %w", err)
	}
	t.Log.Info("telegram authorized", "username", me.Username)
	return nil
}

func (t *TelegramChannel) Start(ctx context.Context) error {
	if err := t.initBot(); err != nil {
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

	t.Log.Info("telegram polling started")
	return nil
}

func (t *TelegramChannel) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	t.Log.Info("telegram stopped")
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

	t.Log.Info("telegram bot commands registered", "count", len(commands))
	return nil
}
