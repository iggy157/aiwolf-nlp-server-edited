package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kano-lab/aiwolf-nlp-server/model"
)

type GameLogger struct {
	deprecatedLogsData map[string]*GameLog
	outputDir          string
	templateFilename   string
}

type GameLog struct {
	id       string
	filename string
	agents   []interface{}
	logs     []string
}

func NewGameLogger(config model.Config) *GameLogger {
	return &GameLogger{
		deprecatedLogsData: make(map[string]*GameLog),
		outputDir:          config.GameLogger.OutputDir,
		templateFilename:   config.GameLogger.Filename,
	}
}

func (g *GameLogger) TrackStartGame(id string, agents []*model.Agent) {
	deprecatedLogData := &GameLog{
		id:   id,
		logs: make([]string, 0),
	}
	for _, agent := range agents {
		deprecatedLogData.agents = append(deprecatedLogData.agents,
			map[string]interface{}{
				"idx":  agent.Idx,
				"team": agent.Team,
				"name": agent.Name,
				"role": agent.Role,
			},
		)
	}
	filename := strings.ReplaceAll(g.templateFilename, "{game_id}", deprecatedLogData.id)
	filename = strings.ReplaceAll(filename, "{timestamp}", fmt.Sprintf("%d", time.Now().Unix()))
	teams := make(map[string]struct{})
	for _, agent := range deprecatedLogData.agents {
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
	deprecatedLogData.filename = filename

	g.deprecatedLogsData[id] = deprecatedLogData
}

func (g *GameLogger) TrackEndGame(id string) {
	if _, exists := g.deprecatedLogsData[id]; exists {
		g.saveDeprecatedLog(id)
		delete(g.deprecatedLogsData, id)
	}
}

func (g *GameLogger) AppendLog(id string, log string) {
	if deprecatedLogData, exists := g.deprecatedLogsData[id]; exists {
		deprecatedLogData.logs = append(deprecatedLogData.logs, log)
		g.saveDeprecatedLog(id)
	}
}

func (g *GameLogger) saveDeprecatedLog(id string) {
	if deprecatedLogData, exists := g.deprecatedLogsData[id]; exists {
		str := strings.Join(deprecatedLogData.logs, "\n")
		if _, err := os.Stat(g.outputDir); os.IsNotExist(err) {
			os.MkdirAll(g.outputDir, 0755)
		}
		filePath := filepath.Join(g.outputDir, fmt.Sprintf("%s.log", deprecatedLogData.filename))
		file, err := os.Create(filePath)
		if err != nil {
			return
		}
		defer file.Close()
		file.WriteString(str)
	}
}
