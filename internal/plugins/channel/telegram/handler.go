package telegram

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

type mediaGroup struct {
	msgs  []*telego.Message
	timer *time.Timer
}

func (t *TelegramChannel) handleMessage(msg *telego.Message) {
	if msg.From == nil {
		return
	}
	senderID := strconv.FormatInt(msg.From.ID, 10)
	if !t.IsAllowed(senderID) {
		t.log.Info("telegram rejected message", "sender", senderID, "username", msg.From.Username)
		return
	}

	if msg.MediaGroupID != "" {
		t.bufferMediaGroup(msg)
		return
	}

	t.dispatchMessage(msg)
}

func (t *TelegramChannel) bufferMediaGroup(msg *telego.Message) {
	t.mgMu.Lock()
	defer t.mgMu.Unlock()
	if t.mgBuffer == nil {
		t.mgBuffer = make(map[string]*mediaGroup)
	}
	gid := msg.MediaGroupID
	g, ok := t.mgBuffer[gid]
	if !ok {
		g = &mediaGroup{}
		t.mgBuffer[gid] = g
		g.timer = time.AfterFunc(500*time.Millisecond, func() { t.flushMediaGroup(gid) })
	}
	g.msgs = append(g.msgs, msg)
}

func (t *TelegramChannel) flushMediaGroup(gid string) {
	t.mgMu.Lock()
	g, ok := t.mgBuffer[gid]
	if !ok {
		t.mgMu.Unlock()
		return
	}
	delete(t.mgBuffer, gid)
	t.mgMu.Unlock()

	if len(g.msgs) == 0 {
		return
	}
	primary := g.msgs[0]
	var allContent []string
	var allBlocks []model.ContentBlock

	for _, m := range g.msgs {
		c, b := t.extractContent(m)
		if c != "" {
			allContent = append(allContent, c)
		}
		allBlocks = append(allBlocks, b...)
	}

	content := ""
	if len(allContent) > 0 {
		content = allContent[0]
		if len(allContent) > 1 {
			seen := map[string]bool{content: true}
			for _, c := range allContent[1:] {
				if !seen[c] {
					seen[c] = true
					content += "\n" + c
				}
			}
		}
	}

	chatID := strconv.FormatInt(primary.Chat.ID, 10)
	_ = t.bus.PublishInbound(t.runCtx, bus.InboundMessage{
		Channel:       telegramChannelName,
		SenderID:      strconv.FormatInt(primary.From.ID, 10),
		ChatID:        chatID,
		Content:       content,
		Timestamp:     time.Unix(int64(primary.Date), 0),
		ContentBlocks: allBlocks,
		Hints: bus.RoutingHints{
			MessageID: primary.MessageID,
		},
		TransportMeta: map[string]any{
			"username":   primary.From.Username,
			"first_name": primary.From.FirstName,
		},
	})
}

func (t *TelegramChannel) dispatchMessage(msg *telego.Message) {
	if t.isSlashCommand(msg) {
		t.handleSlashCommand(msg)
		return
	}
	content, blocks := t.extractContent(msg)
	if content == "" && len(blocks) == 0 {
		return
	}
	chatID := strconv.FormatInt(msg.Chat.ID, 10)
	_ = t.bus.PublishInbound(t.runCtx, bus.InboundMessage{
		Channel:       telegramChannelName,
		SenderID:      strconv.FormatInt(msg.From.ID, 10),
		ChatID:        chatID,
		Content:       content,
		Timestamp:     time.Unix(int64(msg.Date), 0),
		ContentBlocks: blocks,
		Hints: bus.RoutingHints{
			MessageID: msg.MessageID,
		},
		TransportMeta: map[string]any{
			"username":   msg.From.Username,
			"first_name": msg.From.FirstName,
		},
	})
}

func (t *TelegramChannel) extractContent(msg *telego.Message) (string, []model.ContentBlock) {
	var parts []string
	var blocks []model.ContentBlock
	if reply := msg.ReplyToMessage; reply != nil {
		parts = append(parts, ExtractReplyContext(reply))
	} else if msg.ExternalReply != nil || msg.Quote != nil {
		extCtx, extBlocks := t.extractExternalReplyContext(msg.ExternalReply, msg.Quote)
		parts = append(parts, extCtx)
		blocks = append(blocks, extBlocks...)
	}

	if label := ForwardOriginLabel(msg); label != "" {
		parts = append(parts, label)
	}

	body := msg.Text
	if body == "" {
		body = msg.Caption
	} else if msg.Caption != "" {
		body = body + "\n" + msg.Caption
	}

	if msg.ForwardOrigin != nil && body == "" {
		parts = append(parts, "[The user forwarded this message without comment. Summarize or process the content above.]")
	}

	if body != "" {
		parts = append(parts, body)
	}

	content := strings.Join(parts, "\n")
	if len(msg.Photo) > 0 {
		photo := msg.Photo[len(msg.Photo)-1]
		data, err := t.downloadFileData(photo.FileID)
		if err != nil {
			t.log.Warn("telegram download photo failed", "file_id", photo.FileID, "err", err)
		} else {
			mediaType := http.DetectContentType(data)
			if mediaType == "application/octet-stream" {
				mediaType = "image/jpeg"
			}
			blocks = append(blocks, model.ContentBlock{
				Type:      model.ContentBlockImage,
				MediaType: mediaType,
				Data:      base64.StdEncoding.EncodeToString(data),
			})
		}
	}
	if msg.Voice != nil {
		if path, err := t.saveFile(msg.Voice.FileID, "voice.ogg"); err != nil {
			t.log.Warn("telegram save voice failed", "err", err)
			content = AppendLine(content, fmt.Sprintf("[Voice message, %ds, download failed]", msg.Voice.Duration))
		} else {
			content = AppendLine(content, "[Voice message saved to: "+path+"]")
		}
	}
	if msg.Audio != nil {
		name := msg.Audio.FileName
		if name == "" {
			name = "audio.mp3"
		}
		if path, err := t.saveFile(msg.Audio.FileID, name); err != nil {
			t.log.Warn("telegram save audio failed", "err", err)
			content = AppendLine(content, fmt.Sprintf("[Audio: %s, download failed]", name))
		} else {
			content = AppendLine(content, "[Audio file saved to: "+path+"]")
		}
	}
	if msg.Video != nil {
		name := msg.Video.FileName
		if name == "" {
			name = "video.mp4"
		}
		if path, err := t.saveFile(msg.Video.FileID, name); err != nil {
			t.log.Warn("telegram save video failed", "err", err)
			content = AppendLine(content, fmt.Sprintf("[Video: %s, download failed]", name))
		} else {
			content = AppendLine(content, "[Video file saved to: "+path+"]")
		}
	}
	if msg.Document != nil {
		name := msg.Document.FileName
		if name == "" {
			name = "document"
		}
		mediaType := msg.Document.MimeType
		if strings.HasPrefix(mediaType, "image/") {
			data, err := t.downloadFileData(msg.Document.FileID)
			if err != nil {
				t.log.Warn("telegram download document failed", "file_id", msg.Document.FileID, "err", err)
				content = AppendLine(content, fmt.Sprintf("[Image document: %s (%s), download failed]", name, mediaType))
			} else {
				blocks = append(blocks, model.ContentBlock{
					Type:      model.ContentBlockImage,
					MediaType: mediaType,
					Data:      base64.StdEncoding.EncodeToString(data),
				})
			}
		} else {
			if path, err := t.saveFile(msg.Document.FileID, name); err != nil {
				t.log.Warn("telegram save document failed", "err", err)
				info := fmt.Sprintf("[File: %s (%s)", name, mediaType)
				if msg.Document.FileSize > 0 {
					info += fmt.Sprintf(", %d bytes", msg.Document.FileSize)
				}
				info += ", download failed]"
				content = AppendLine(content, info)
			} else {
				content = AppendLine(content, "[File saved to: "+path+"]")
			}
		}
	}
	return content, blocks
}

func (t *TelegramChannel) extractExternalReplyContext(ext *telego.ExternalReplyInfo, quote *telego.TextQuote) (string, []model.ContentBlock) {
	var b strings.Builder
	var blocks []model.ContentBlock
	b.WriteString("[Replying to")
	if ext != nil {
		switch o := ext.Origin.(type) {
		case *telego.MessageOriginUser:
			name := strings.TrimSpace(o.SenderUser.FirstName + " " + o.SenderUser.LastName)
			b.WriteString(" " + name)
		case *telego.MessageOriginHiddenUser:
			b.WriteString(" " + o.SenderUserName)
		case *telego.MessageOriginChat:
			b.WriteString(" chat: " + o.SenderChat.Title)
		case *telego.MessageOriginChannel:
			b.WriteString(" channel: " + o.Chat.Title)
		}
		if len(ext.Photo) > 0 {
			photo := ext.Photo[len(ext.Photo)-1]
			data, err := t.downloadFileData(photo.FileID)
			if err != nil {
				t.log.Warn("telegram download external reply photo failed", "err", err)
				b.WriteString("\n[Photo, download failed]")
			} else {
				mediaType := http.DetectContentType(data)
				if mediaType == "application/octet-stream" {
					mediaType = "image/jpeg"
				}
				blocks = append(blocks, model.ContentBlock{
					Type:      model.ContentBlockImage,
					MediaType: mediaType,
					Data:      base64.StdEncoding.EncodeToString(data),
				})
			}
		}
		if ext.Voice != nil {
			b.WriteString("\n[Voice message]")
		}
		if ext.Audio != nil {
			b.WriteString("\n[Audio: " + ext.Audio.FileName + "]")
		}
		if ext.Document != nil {
			b.WriteString("\n[File: " + ext.Document.FileName + "]")
		}
		if ext.Video != nil {
			b.WriteString("\n[Video]")
		}
		if ext.Sticker != nil {
			b.WriteString("\n[Sticker: " + ext.Sticker.Emoji + "]")
		}
	}
	b.WriteString("]")
	if quote != nil && quote.Text != "" {
		b.WriteString("\n" + quote.Text)
	}
	return b.String(), blocks
}

func (t *TelegramChannel) saveFile(fileID, name string) (string, error) {
	if t.workspace == "" {
		return "", fmt.Errorf("workspace not configured")
	}
	data, err := t.downloadFileData(fileID)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(t.workspace, "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create uploads dir: %w", err)
	}
	name = fmt.Sprintf("%d_%s", time.Now().UnixMilli(), sanitizeUploadName(name, "file"))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	t.log.Debug("telegram saved file", "path", path, "bytes", len(data))
	return path, nil
}

func sanitizeUploadName(name, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fallback
	}
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return fallback
	}
	return name
}

func (t *TelegramChannel) downloadFileData(fileID string) ([]byte, error) {
	if t.bot == nil {
		return nil, fmt.Errorf("telegram bot not initialized")
	}
	file, err := t.bot.GetFile(t.runCtx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, fmt.Errorf("get telegram file: %w", err)
	}
	downloadURL := t.bot.FileDownloadURL(file.FilePath)
	client := t.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("download telegram file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download telegram file: unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read telegram file body: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("telegram file is empty")
	}
	return data, nil
}

func (t *TelegramChannel) PreProcessFeedback(chatID int64, messageID int) {
	switch t.feedback {
	case "debug":
		t.sendReactionAndTyping(chatID, messageID, "👀")
	case "normal":
		t.sendReactionAndTyping(chatID, messageID, "👍")
	case "minimal":
		t.sendTyping(chatID)
	case "silent":
	}
}

func (t *TelegramChannel) sendReactionAndTyping(chatID int64, messageID int, emoji string) {
	go func() {
		t.sendReaction(chatID, messageID, emoji)
	}()
	go func() {
		t.sendTyping(chatID)
	}()
}

func (t *TelegramChannel) sendReaction(chatID int64, messageID int, emoji string) {
	if t.bot == nil || messageID <= 0 {
		return
	}
	err := t.bot.SetMessageReaction(t.runCtx, &telego.SetMessageReactionParams{
		ChatID:    tu.ID(chatID),
		MessageID: messageID,
		Reaction:  []telego.ReactionType{tu.ReactionEmoji(emoji)},
	})
	if err != nil {
		t.log.Warn("telegram sendReaction failed", "err", err)
	}
}

func (t *TelegramChannel) sendTyping(chatID int64) {
	if t.bot == nil {
		return
	}
	err := t.bot.SendChatAction(t.runCtx, tu.ChatAction(tu.ID(chatID), telego.ChatActionTyping))
	if err != nil {
		t.log.Warn("telegram sendTyping failed", "err", err)
	}
}
