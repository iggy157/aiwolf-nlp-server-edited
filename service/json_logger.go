package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"slices"

	"github.com/iggy157/aiwolf-nlp-server-edited-edited/model"
)

type JSONLogger struct {
	config model.JSONLoggerConfig
	data   sync.Map
}

type JSONLog struct {
	id           string
	filename     string
	agents       []any
	winSide      model.Team
	entries      []any
	timestampMap sync.Map
	requestMap   sync.Map
	mu           sync.Mutex
}

func NewJSONLogger(config model.Config) *JSONLogger {
	return &JSONLogger{
		config: config.JSONLogger,
	}
}

func (j *JSONLogger) TrackStartGame(id string, agents []*model.Agent) {
	data := &JSONLog{
		id:      id,
		agents:  make([]any, 0),
		entries: make([]any, 0),
		winSide: model.T_NONE,
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

	filename := strings.ReplaceAll(j.config.Filename, "{game_id}", data.id)
	filename = strings.ReplaceAll(filename, "{timestamp}", fmt.Sprintf("%d", time.Now().Unix()))

	teams := make([]string, 0)
	for _, agent := range data.agents {
		team := agent.(map[string]any)["team"].(string)
		teams = append(teams, team)
	}
	sort.Strings(teams)
	filename = strings.ReplaceAll(filename, "{teams}", strings.Join(teams, "_"))

	data.filename = filename
	j.data.Store(id, data)
}

func (j *JSONLogger) TrackEndGame(id string, winSide model.Team) {
	if dataInterface, exists := j.data.Load(id); exists {
		data := dataInterface.(*JSONLog)
		data.winSide = winSide
		j.saveGameData(id)
		j.data.Delete(id)
	}
}

func (j *JSONLogger) TrackStartRequest(id string, agent model.Agent, packet model.Packet) {
	if dataInterface, exists := j.data.Load(id); exists {
		data := dataInterface.(*JSONLog)
		data.timestampMap.Store(agent.OriginalName, time.Now().UnixNano())
		data.requestMap.Store(agent.OriginalName, packet)
	}
}

func (j *JSONLogger) TrackEndRequest(id string, agent model.Agent, response string, err error) {
	if dataInterface, exists := j.data.Load(id); exists {
		data := dataInterface.(*JSONLog)
		timestamp := time.Now().UnixNano()

		entry := map[string]any{
			"agent":              agent.String(),
			"response_timestamp": timestamp / 1e6,
		}

		if requestTimestampInterface, exists := data.timestampMap.LoadAndDelete(agent.OriginalName); exists {
			entry["request_timestamp"] = requestTimestampInterface.(int64) / 1e6
		}

		if requestInterface, exists := data.requestMap.LoadAndDelete(agent.OriginalName); exists {
			if jsonData, marshalErr := json.Marshal(requestInterface); marshalErr == nil {
				entry["request"] = string(jsonData)
			}
		}

		if response != "" {
			entry["response"] = response
		}

		if err != nil {
			entry["error"] = err.Error()
		}

		data.mu.Lock()
		data.entries = append(data.entries, entry)
		data.mu.Unlock()

		j.saveGameData(id)
	}
}

func (j *JSONLogger) saveGameData(id string) {
	if dataInterface, exists := j.data.Load(id); exists {
		data := dataInterface.(*JSONLog)

		data.mu.Lock()
		game := map[string]any{
			"game_id":  id,
			"win_side": data.winSide,
			"agents":   data.agents,
			"entries":  slices.Clone(data.entries),
		}
		data.mu.Unlock()

		jsonData, err := json.Marshal(game)
		if err != nil {
			return
		}

		if _, err := os.Stat(j.config.OutputDir); os.IsNotExist(err) {
			os.MkdirAll(j.config.OutputDir, 0755)
		}

		filePath := filepath.Join(j.config.OutputDir, fmt.Sprintf("%s.json", data.filename))
		file, err := os.Create(filePath)
		if err != nil {
			return
		}
		defer file.Close()

		file.Write(jsonData)
	}
}
