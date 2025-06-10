package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/aiwolfdial/aiwolf-nlp-server/util"
	"github.com/gorilla/websocket"
)

type RealtimeBroadcaster struct {
	authentication bool
	config         model.RealtimeBroadcasterConfig
	upgrader       websocket.Upgrader
	clients        *sync.Map
	data           sync.Map
}

type RealtimeBroadcasterLog struct {
	id       string
	filename string
	agents   []any
	logs     []string
	logsMu   sync.Mutex
}

type ClientConnection struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func NewRealtimeBroadcaster(config model.Config) *RealtimeBroadcaster {
	return &RealtimeBroadcaster{
		authentication: config.Server.Authentication.Enable,
		config:         config.RealtimeBroadcaster,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		clients: &sync.Map{},
	}
}

func (rb *RealtimeBroadcaster) TrackStartGame(id string, agents []*model.Agent) {
	data := &RealtimeBroadcasterLog{
		id:     id,
		logs:   make([]string, 0),
		agents: make([]any, 0),
	}

	for _, agent := range agents {
		data.agents = append(data.agents,
			map[string]any{
				"idx":  agent.Idx,
				"team": agent.TeamName,
				"name": agent.OriginalName,
				"role": agent.Role,
			},
		)
	}

	filename := strings.ReplaceAll(rb.config.Filename, "{game_id}", data.id)
	filename = strings.ReplaceAll(filename, "{timestamp}", fmt.Sprintf("%d", time.Now().Unix()))

	teams := make([]string, 0)
	for _, agent := range data.agents {
		team := agent.(map[string]any)["team"].(string)
		teams = append(teams, team)
	}
	sort.Strings(teams)
	filename = strings.ReplaceAll(filename, "{teams}", strings.Join(teams, "_"))

	data.filename = filename
	rb.data.Store(id, data)
}

func (rb *RealtimeBroadcaster) TrackEndGame(id string) {
	if _, exists := rb.data.Load(id); exists {
		rb.saveLog(id)
		rb.data.Delete(id)
	}
}

func (rb *RealtimeBroadcaster) Broadcast(packet model.BroadcastPacket) {
	if jsonData, marshalErr := json.Marshal(packet); marshalErr == nil {
		rb.appendLog(packet.Id, string(jsonData))
	}

	go func() {
		time.Sleep(rb.config.Delay)
		var disconnectedClients []*ClientConnection

		rb.clients.Range(func(key, value any) bool {
			clientConn := key.(*ClientConnection)

			clientConn.mu.Lock()
			err := clientConn.conn.WriteJSON(packet)
			clientConn.mu.Unlock()

			if err != nil {
				slog.Warn("クライアントへのメッセージ送信に失敗しました", "error", err)
				disconnectedClients = append(disconnectedClients, clientConn)
			}
			return true
		})

		for _, clientConn := range disconnectedClients {
			clientConn.conn.Close()
			rb.clients.Delete(clientConn)
		}

		slog.Info("リアルタイムブロードキャストを送信しました", "packet", packet)
	}()
}

func (rb *RealtimeBroadcaster) HandleConnections(w http.ResponseWriter, r *http.Request) {
	if rb.authentication {
		token := r.URL.Query().Get("token")
		if token != "" {
			if !util.IsValidReceiver(os.Getenv("SECRET_KEY"), token) {
				slog.Warn("トークンが無効です")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		} else {
			token = strings.ReplaceAll(r.Header.Get("Authorization"), "Bearer ", "")
			if !util.IsValidReceiver(os.Getenv("SECRET_KEY"), token) {
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

	clientConn := &ClientConnection{
		conn: ws,
	}
	rb.clients.Store(clientConn, nil)
	defer rb.clients.Delete(clientConn)

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			slog.Error("クライアントの読み込みに失敗しました", "error", err)
			break
		}
	}
}

func (rb *RealtimeBroadcaster) appendLog(id string, log string) {
	if dataInterface, exists := rb.data.Load(id); exists {
		data := dataInterface.(*RealtimeBroadcasterLog)

		data.logsMu.Lock()
		data.logs = append(data.logs, log)
		logsCopy := make([]string, len(data.logs))
		copy(logsCopy, data.logs)
		data.logsMu.Unlock()

		rb.saveLogWithData(data.filename, logsCopy)
	}
}

func (rb *RealtimeBroadcaster) saveLog(id string) {
	if dataInterface, exists := rb.data.Load(id); exists {
		data := dataInterface.(*RealtimeBroadcasterLog)

		data.logsMu.Lock()
		logsCopy := make([]string, len(data.logs))
		copy(logsCopy, data.logs)
		filename := data.filename
		data.logsMu.Unlock()

		rb.saveLogWithData(filename, logsCopy)
	}
}

func (rb *RealtimeBroadcaster) saveLogWithData(filename string, logs []string) {
	str := strings.Join(logs, "\n")

	if _, err := os.Stat(rb.config.OutputDir); os.IsNotExist(err) {
		os.MkdirAll(rb.config.OutputDir, 0755)
	}

	filePath := filepath.Join(rb.config.OutputDir, fmt.Sprintf("%s.jsonl", filename))
	file, err := os.Create(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	file.WriteString(str)
}
