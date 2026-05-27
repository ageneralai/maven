package telegram

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/session"
	"github.com/ageneralai/maven/internal/sessionid"
	"github.com/ageneralai/maven/internal/slashkind"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

func (t *TelegramChannel) loadSlashCommands() {
	t.slashCommands = make(map[string]Command)
	root := t.telegramRoot()
	if root == "" {
		t.log.Info("telegram skip slash command load: telegram root is not configured")
		return
	}
	dir := filepath.Join(root, "slashes")
	cmds, err := LoadCommands(dir)
	if err != nil {
		t.log.Error("telegram load slash commands", "err", err)
		return
	}
	for _, cmd := range cmds {
		t.slashCommands[cmd.Name] = cmd
	}
	if len(t.slashCommands) > 0 {
		t.log.Info("telegram loaded slash commands", "count", len(t.slashCommands), "dir", dir)
		return
	}
	t.log.Info("telegram no slash commands found", "dir", dir)
}

func (t *TelegramChannel) registeredBotCommands() []telego.BotCommand {
	descriptions := map[string]string{
		"new": "Start a fresh session",
	}
	for name, cmd := range t.slashCommands {
		if strings.TrimSpace(name) == "" {
			continue
		}
		descriptions[name] = telegramCommandDescription(name, cmd.Description)
	}

	names := make([]string, 0, len(descriptions))
	for name := range descriptions {
		names = append(names, name)
	}
	sort.Strings(names)

	commands := make([]telego.BotCommand, 0, len(names))
	for _, name := range names {
		commands = append(commands, telego.BotCommand{
			Command:     name,
			Description: descriptions[name],
		})
	}
	return commands
}

func telegramCommandDescription(name, desc string) string {
	desc = strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(desc))
	if desc == "" {
		desc = "Run /" + name
	}
	if len(desc) > 256 {
		desc = desc[:256]
	}
	return desc
}

func (t *TelegramChannel) isSlashCommand(msg *telego.Message) bool {
	if msg.Text == "" {
		return false
	}
	for _, e := range msg.Entities {
		if e.Type == "bot_command" && e.Offset == 0 {
			return true
		}
	}
	return false
}

// sendBotReply delivers text to a chat on a best-effort basis; errors are logged but not propagated.
func (t *TelegramChannel) sendBotReply(chatID int64, text string) {
	if t.bot == nil {
		return
	}
	if _, err := t.bot.SendMessage(t.runCtx, tu.Message(tu.ID(chatID), text)); err != nil {
		t.log.Error("telegram sendMessage failed", "err", err)
	}
}

func (t *TelegramChannel) handleSlashCommand(msg *telego.Message) {
	parts := strings.Fields(msg.Text)
	if len(parts) == 0 {
		return
	}
	cmdName := strings.TrimPrefix(parts[0], "/")
	args := ""
	if len(parts) > 1 {
		args = strings.Join(parts[1:], " ")
	}

	if t.handleBuiltinSlashCommand(msg, cmdName) {
		return
	}

	cmd, ok := t.slashCommands[cmdName]
	if !ok {
		t.sendBotReply(msg.Chat.ID, "Unknown command: /"+cmdName)
		return
	}

	switch cmd.Type {
	case slashkind.CommandKindLocal:
		resp := t.executeLocalCommand(cmd, args)
		t.sendBotReply(msg.Chat.ID, resp)
	case slashkind.CommandKindAgent, slashkind.CommandKindPipeline:
		content := t.composeAgentCommandContent(cmd, cmdName, args)
		if strings.TrimSpace(content) == "" {
			t.sendBotReply(msg.Chat.ID, "Command is not configured with executable content.")
			return
		}

		hints := bus.RoutingHints{
			SlashCommand: cmdName,
			SlashType:    string(cmd.Type),
			SlashArgs:    args,
			MessageID:    msg.MessageID,
			ForceSync:    !cmd.Streaming,
		}
		if cmd.Session == session.SessionModeIsolated {
			hints.SessionMode = session.SessionModeIsolated
		}
		_ = t.bus.PublishInbound(t.runCtx, bus.InboundMessage{
			Channel:   telegramChannelName,
			SenderID:  strconv.FormatInt(msg.From.ID, 10),
			ChatID:    strconv.FormatInt(msg.Chat.ID, 10),
			Content:   content,
			Timestamp: time.Now(),
			Hints:     hints,
		})
	}
}

func (t *TelegramChannel) handleBuiltinSlashCommand(msg *telego.Message, cmdName string) bool {
	if cmdName != "new" {
		return false
	}

	_ = t.bus.PublishInbound(t.runCtx, bus.InboundMessage{
		Channel:   telegramChannelName,
		SenderID:  strconv.FormatInt(msg.From.ID, 10),
		ChatID:    strconv.FormatInt(msg.Chat.ID, 10),
		Timestamp: time.Now(),
		Hints: bus.RoutingHints{
			BuiltinCommand: "new",
			ForceSync:      true,
			MessageID:      msg.MessageID,
		},
	})
	return true
}

func (t *TelegramChannel) executeLocalCommand(cmd Command, args string) string {
	sessionID := sessionid.New(sessionid.KindIsolated, telegramChannelName).String()
	switch cmd.Name {
	case "status":
		return "✅ Bot is running"
	case "help":
		lines := t.helpLines()
		return strings.Join(append([]string{"Available commands:"}, lines...), "\n")
	default:
		if cmd.Handler != "" {
			return t.executeHandler(cmd.Handler, args, sessionID)
		}
		if cmd.Prompt != "" {
			return cmd.Prompt
		}
		return "✅ Done"
	}
}

func (t *TelegramChannel) helpLines() []string {
	lines := []string{"/new - Start a fresh session"}
	for name, cmd := range t.slashCommands {
		line := "/" + name
		if desc := strings.TrimSpace(cmd.Description); desc != "" {
			line += " - " + desc
		}
		lines = append(lines, line)
	}
	sort.Strings(lines)
	return lines
}

func (t *TelegramChannel) composeAgentCommandContent(cmd Command, cmdName, args string) string {
	if cmd.PassThrough {
		content := "/" + cmdName
		if args != "" {
			content += " " + args
		}
		return content
	}

	prompt := strings.TrimSpace(cmd.Prompt)
	if args == "" {
		return prompt
	}
	if prompt == "" {
		return args
	}
	return prompt + "\n\nUser input:\n" + args
}

func (t *TelegramChannel) executeHandler(handler, args, sessionID string) string {
	handlerPath := filepath.Join(t.telegramRoot(), "handlers", handler)
	if !filepath.IsAbs(handler) {
		handler = handlerPath
	}

	cmd := exec.Command(handler, sessionID, args)
	cmd.Env = append(os.Environ(),
		"WORKSPACE="+t.workspace,
		"SESSION_ID="+sessionID,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("❌ Handler failed: %v\n%s", err, output)
	}
	return string(output)
}
