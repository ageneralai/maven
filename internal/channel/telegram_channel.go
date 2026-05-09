package channel

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel/telegram"
	"github.com/ageneralai/maven/internal/config"
	mavenlog "github.com/ageneralai/maven/pkg/log"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const telegramChannelName = "telegram"

type TelegramChannel struct {
	BaseChannel
	token      string
	bot        *telego.Bot
	proxy      string
	httpClient *http.Client
	cancel     context.CancelFunc
	feedback   string // "debug", "normal", "minimal", "silent"
	streaming  bool
	workspace  string // workspace root for file saving
	rootDir    string // telegram assets root, default <workspace>/.telegram
	caps       CapabilitySet

	mgMu     sync.Mutex
	mgBuffer map[string]*mediaGroup

	slashCommands map[string]telegram.Command
}

func NewTelegramChannel(cfg config.TelegramConfig, workspace string, lg mavenlog.PrintLogger, b *bus.MessageBus) (*TelegramChannel, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram token is required")
	}
	feedback := cfg.Feedback
	if feedback == "" {
		feedback = "normal"
	}
	ch := &TelegramChannel{
		BaseChannel: NewBaseChannel(telegramChannelName, b, cfg.AllowFrom, lg),
		token:       cfg.Token,
		proxy:       cfg.Proxy,
		httpClient:  http.DefaultClient,
		feedback:    feedback,
		streaming:   cfg.Streaming,
		rootDir:     strings.TrimSpace(cfg.RootDir),
		workspace:   strings.TrimSpace(workspace),
		caps: CapabilitySet{
			Reactions:  true,
			FileUpload: true,
		},
	}
	return ch, nil
}

func (t *TelegramChannel) Capabilities() CapabilitySet { return t.caps }

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

	var client *http.Client
	if t.proxy != "" {
		proxyURL, err := url.Parse(t.proxy)
		if err != nil {
			return fmt.Errorf("parse proxy url: %w", err)
		}
		client = &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		}
	} else {
		client = http.DefaultClient
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
	t.log.Printf("[telegram] authorized as @%s", me.Username)
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

	t.log.Printf("[telegram] polling started")
	return nil
}

func (t *TelegramChannel) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}
	t.log.Printf("[telegram] stopped")
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

	t.log.Printf("[telegram] registered %d bot commands", len(commands))
	return nil
}
