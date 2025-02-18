package service

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/kano-lab/aiwolf-nlp-server/model"
	"github.com/kano-lab/aiwolf-nlp-server/util"
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

func (rb *RealtimeBroadcaster) Broadcast(id string, data interface{}) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for client := range rb.clients {
		err := client.WriteJSON(map[string]interface{}{
			"id":   id,
			"data": data,
		})
		if err != nil {
			rb.mu.Lock()
			delete(rb.clients, client)
			rb.mu.Unlock()
			client.Close()
		}
	}
}

func (rb *RealtimeBroadcaster) HandleConnections(w http.ResponseWriter, r *http.Request) {
	if rb.config.Server.Authentication.Enable {
		token := strings.ReplaceAll(r.Header.Get("Authorization"), "Bearer ", "")
		if !util.IsValidReceiver(rb.config.Server.Authentication.Secret, token) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	ws, err := rb.upgrader.Upgrade(w, r, nil)
	if err != nil {
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
			break
		}
	}
}
