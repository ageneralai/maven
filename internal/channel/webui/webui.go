package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	chann "github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/config"
	mavenlog "github.com/ageneralai/maven/pkg/log"
	"github.com/coder/websocket"
)

//go:embed static
var staticFiles embed.FS

const webUIChannelName = "webui"

type wsMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

type wsClient struct {
	conn *websocket.Conn
	id   string
}

type WebUIChannel struct {
	chann.BaseChannel
	port    int
	server  *http.Server
	clients sync.Map
	nextID  atomic.Int64
}

func NewWebUIChannel(cfg config.WebUIConfig, gwCfg config.GatewayConfig, lg mavenlog.PrintLogger, b *bus.MessageBus) (*WebUIChannel, error) {
	port := gwCfg.Port
	if port == 0 {
		port = config.DefaultPort
	}

	ch := &WebUIChannel{
		BaseChannel: chann.NewBaseChannel(webUIChannelName, b, cfg.AllowFrom, lg),
		port:        port,
	}
	return ch, nil
}

func (w *WebUIChannel) Start(ctx context.Context) error {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("embed static fs: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/ws", w.handleWS)

	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", w.port),
		Handler: mux,
	}

	go func() {
		w.Log.Printf("[webui] listening on :%d", w.port)
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			w.Log.Printf("[webui] server error: %v", err)
		}
	}()

	return nil
}

func (w *WebUIChannel) handleWS(wr http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(wr, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		w.Log.Printf("[webui] websocket accept error: %v", err)
		return
	}

	clientID := fmt.Sprintf("webui-%d", w.nextID.Add(1))
	client := &wsClient{conn: conn, id: clientID}
	w.clients.Store(clientID, client)
	w.Log.Printf("[webui] client connected: %s", clientID)

	defer func() {
		w.clients.Delete(clientID)
		_ = conn.CloseNow()
		w.Log.Printf("[webui] client disconnected: %s", clientID)
	}()

	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		if msg.Type != "message" || msg.Content == "" {
			continue
		}

		if !w.IsAllowed(clientID) {
			w.Log.Printf("[webui] rejected message from %s", clientID)
			continue
		}

		_ = w.Bus.PublishInbound(r.Context(), bus.InboundMessage{
			Channel:   webUIChannelName,
			SenderID:  clientID,
			ChatID:    clientID,
			Content:   msg.Content,
			Timestamp: time.Now(),
		})
	}
}

func (w *WebUIChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	data, err := json.Marshal(wsMessage{
		Type:    "message",
		Content: msg.Content,
	})
	if err != nil {
		return err
	}

	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client, ok := w.clients.Load(msg.ChatID)
	if !ok {
		// Broadcast to all clients if no specific target
		w.clients.Range(func(key, value any) bool {
			c := value.(*wsClient)
			_ = c.conn.Write(writeCtx, websocket.MessageText, data)
			return true
		})
		return nil
	}

	c := client.(*wsClient)
	return c.conn.Write(writeCtx, websocket.MessageText, data)
}

func (w *WebUIChannel) Capabilities() chann.CapabilitySet {
	return chann.CapabilitySet{}
}

func (w *WebUIChannel) Stop() error {
	if w.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.server.Shutdown(ctx); err != nil {
			w.Log.Printf("[webui] shutdown error: %v", err)
		}
	}
	w.clients.Range(func(key, value any) bool {
		c := value.(*wsClient)
		_ = c.conn.CloseNow()
		return true
	})
	w.Log.Printf("[webui] stopped")
	return nil
}
