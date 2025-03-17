package service

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/aiwolfdial/aiwolf-nlp-server/util"
	"github.com/gorilla/websocket"
)

type RealtimeBroadcaster struct {
	config   model.Config
	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]bool
	mu       sync.Mutex
}

func NewRealtimeBroadcaster(config model.Config) *RealtimeBroadcaster {
	return &RealtimeBroadcaster{
		config: config,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		clients: make(map[*websocket.Conn]bool),
	}
}

func (rb *RealtimeBroadcaster) Broadcast(packet model.BroadcastPacket) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for client := range rb.clients {
		err := client.WriteJSON(packet)
		if err != nil {
			rb.mu.Lock()
			delete(rb.clients, client)
			rb.mu.Unlock()
			client.Close()
		}
	}
	slog.Info("リアルタイムブロードキャストを送信しました", "packet", packet)
}

func (rb *RealtimeBroadcaster) HandleConnections(w http.ResponseWriter, r *http.Request) {
	if rb.config.Server.Authentication.Enable {
		token := r.URL.Query().Get("token")
		if token != "" {
			if !util.IsValidReceiver(rb.config.Server.Authentication.Secret, token) {
				slog.Warn("トークンが無効です")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		} else {
			token = strings.ReplaceAll(r.Header.Get("Authorization"), "Bearer ", "")
			if !util.IsValidReceiver(rb.config.Server.Authentication.Secret, token) {
				slog.Warn("トークンが無効です")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
	}

	ws, err := rb.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("クライアントのアップグレードに失敗しました", "error", err)
		return
	}
	defer ws.Close()

	rb.mu.Lock()
	rb.clients[ws] = true
	rb.mu.Unlock()

	defer func() {
		rb.mu.Lock()
		delete(rb.clients, ws)
		rb.mu.Unlock()
	}()

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			slog.Error("クライアントの読み込みに失敗しました", "error", err)
			break
		}
	}
}
