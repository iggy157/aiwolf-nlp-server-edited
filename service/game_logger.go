package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

type GameLogger struct {
	data             map[string]*GameLog
	outputDir        string
	templateFilename string
}

type GameLog struct {
	id       string
	filename string
	agents   []interface{}
	logs     []string
}

func NewGameLogger(config model.Config) *GameLogger {
	return &GameLogger{
		data:             make(map[string]*GameLog),
		outputDir:        config.GameLogger.OutputDir,
		templateFilename: config.GameLogger.Filename,
	}
}

func (g *GameLogger) TrackStartGame(id string, agents []*model.Agent) {
	data := &GameLog{
		id:   id,
		logs: make([]string, 0),
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
	filename := strings.ReplaceAll(g.templateFilename, "{game_id}", data.id)
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

	g.data[id] = data
}

func (g *GameLogger) TrackEndGame(id string) {
	if _, exists := g.data[id]; exists {
		g.saveLog(id)
		delete(g.data, id)
	}
}

func (g *GameLogger) AppendLog(id string, log string) {
	if data, exists := g.data[id]; exists {
		data.logs = append(data.logs, log)
		g.saveLog(id)
	}
}

func (g *GameLogger) saveLog(id string) {
	if data, exists := g.data[id]; exists {
		str := strings.Join(data.logs, "\n")
		if _, err := os.Stat(g.outputDir); os.IsNotExist(err) {
			os.MkdirAll(g.outputDir, 0755)
		}
		filePath := filepath.Join(g.outputDir, fmt.Sprintf("%s.log", data.filename))
		file, err := os.Create(filePath)
		if err != nil {
			return
		}
		defer file.Close()
		file.WriteString(str)
	}
}
