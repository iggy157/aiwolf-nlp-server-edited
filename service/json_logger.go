package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kano-lab/aiwolf-nlp-server/model"
)

type JSONLogger struct {
	data             map[string]*JSONLog
	outputDir        string
	templateFilename string
	endGameStatus    map[string]bool
	upgrader         websocket.Upgrader
	clients          map[*websocket.Conn]bool
	clientsMu        sync.Mutex
}

type JSONLog struct {
	id           string
	filename     string
	agents       []interface{}
	winSide      model.Team
	entries      []interface{}
	timestampMap map[string]int64
	requestMap   map[string]interface{}
}

func NewJSONLogger(config model.Config) *JSONLogger {
	return &JSONLogger{
		data:             make(map[string]*JSONLog),
		outputDir:        config.JSONLogger.OutputDir,
		templateFilename: config.JSONLogger.Filename,
		endGameStatus:    make(map[string]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		clients: make(map[*websocket.Conn]bool),
	}
}

func (j *JSONLogger) TrackStartGame(id string, agents []*model.Agent) {
	data := &JSONLog{
		id:           id,
		agents:       make([]interface{}, 0),
		entries:      make([]interface{}, 0),
		timestampMap: make(map[string]int64),
		requestMap:   make(map[string]interface{}),
		winSide:      model.T_NONE,
	}
	for _, agent := range agents {
		data.agents = append(data.agents,
			map[string]interface{}{
				"idx":  agent.Idx,
				"team": agent.Team,
				"name": agent.Name,
				"role": agent.Role,
			},
		)
	}
	filename := strings.ReplaceAll(j.templateFilename, "{game_id}", data.id)
	filename = strings.ReplaceAll(filename, "{timestamp}", fmt.Sprintf("%d", time.Now().Unix()))
	teams := make(map[string]struct{})
	for _, agent := range data.agents {
		team := agent.(map[string]interface{})["team"].(string)
		teams[team] = struct{}{}
	}
	teamStr := ""
	for team := range teams {
		if teamStr != "" {
			teamStr += "_"
		}
		teamStr += team
	}
	filename = strings.ReplaceAll(filename, "{teams}", teamStr)
	data.filename = filename

	j.data[id] = data
	j.endGameStatus[id] = false

	j.broadcastToClients(map[string]interface{}{
		"type":    "game_start",
		"game_id": id,
		"agents":  data.agents,
	})
}

func (j *JSONLogger) TrackEndGame(id string, winSide model.Team) {
	if data, exists := j.data[id]; exists {
		data.winSide = winSide
		j.endGameStatus[id] = true

		j.broadcastToClients(map[string]interface{}{
			"type":     "game_end",
			"game_id":  id,
			"win_side": winSide,
		})

		j.saveGameData(id)
	}
}

func (j *JSONLogger) TrackStartRequest(id string, agent model.Agent, packet model.Packet) {
	if data, exists := j.data[id]; exists {
		data.timestampMap[agent.Name] = time.Now().UnixNano()
		data.requestMap[agent.Name] = packet
	}
}

func (j *JSONLogger) TrackEndRequest(id string, agent model.Agent, response string, err error) {
	if data, exists := j.data[id]; exists {
		timestamp := time.Now().UnixNano()
		entry := map[string]interface{}{
			"agent":              agent.String(),
			"request_timestamp":  data.timestampMap[agent.Name] / 1e6,
			"response_timestamp": timestamp / 1e6,
		}
		if request, ok := data.requestMap[agent.Name]; ok {
			jsonData, err := json.Marshal(request)
			if err == nil {
				entry["request"] = string(jsonData)
			}
		}
		if response != "" {
			entry["response"] = response
		}
		if err != nil {
			entry["error"] = err.Error()
		}
		data.entries = append(data.entries, entry)
		delete(data.timestampMap, agent.Name)
		delete(data.requestMap, agent.Name)

		j.broadcastToClients(map[string]interface{}{
			"type":    "log_entry",
			"game_id": id,
			"entry":   entry,
		})

		j.saveGameData(id)
	}
}

func (j *JSONLogger) saveGameData(id string) {
	if data, exists := j.data[id]; exists {
		game := map[string]interface{}{
			"game_id":  id,
			"win_side": data.winSide,
			"agents":   data.agents,
			"entries":  data.entries,
		}
		jsonData, err := json.Marshal(game)
		if err != nil {
			return
		}
		if _, err := os.Stat(j.outputDir); os.IsNotExist(err) {
			os.Mkdir(j.outputDir, 0755)
		}
		filePath := filepath.Join(j.outputDir, fmt.Sprintf("%s.json", data.filename))
		file, err := os.Create(filePath)
		if err != nil {
			return
		}
		defer file.Close()
		file.Write(jsonData)
	}
}

func (j *JSONLogger) broadcastToClients(data interface{}) {
	j.clientsMu.Lock()
	defer j.clientsMu.Unlock()

	for client := range j.clients {
		err := client.WriteJSON(data)
		if err != nil {
			// エラーが発生した場合はクライアントを削除
			j.clientsMu.Lock()
			delete(j.clients, client)
			j.clientsMu.Unlock()
			client.Close()
		}
	}
}

func (j *JSONLogger) HandleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := j.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	// 新しいクライアントを追加
	j.clientsMu.Lock()
	j.clients[ws] = true
	j.clientsMu.Unlock()

	// クライアントが切断したときの処理
	defer func() {
		j.clientsMu.Lock()
		delete(j.clients, ws)
		j.clientsMu.Unlock()
	}()

	// クライアントからのメッセージを待ち受ける（必要に応じて）
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}
