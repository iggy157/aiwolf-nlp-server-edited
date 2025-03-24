package logic

import (
	"fmt"
	"log/slog"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/aiwolfdial/aiwolf-nlp-server/service"
	"github.com/aiwolfdial/aiwolf-nlp-server/util"
	"github.com/oklog/ulid/v2"
)

type Game struct {
	ID                           string
	Agents                       []*model.Agent
	IsFinished                   bool
	config                       *model.Config
	setting                      *model.Setting
	currentDay                   int
	gameStatuses                 map[int]*model.GameStatus
	lastTalkIdxMap               map[*model.Agent]int
	lastWhisperIdxMap            map[*model.Agent]int
	jsonLogger                   *service.JSONLogger
	gameLogger                   *service.GameLogger
	realtimeBroadcaster          *service.RealtimeBroadcaster
	realtimeBroadcasterPacketIdx int
}

func NewGame(config *model.Config, settings *model.Setting, conns []model.Connection) *Game {
	id := ulid.Make().String()
	agents := util.CreateAgents(conns, settings.RoleNumMap)
	gameStatus := model.NewInitializeGameStatus(agents)
	gameStatuses := make(map[int]*model.GameStatus)
	gameStatuses[0] = &gameStatus
	slog.Info("ゲームを作成しました", "id", id)
	return &Game{
		ID:                id,
		Agents:            agents,
		IsFinished:        false,
		config:            config,
		setting:           settings,
		currentDay:        0,
		gameStatuses:      gameStatuses,
		lastTalkIdxMap:    make(map[*model.Agent]int),
		lastWhisperIdxMap: make(map[*model.Agent]int),
	}
}

func NewGameWithRole(config *model.Config, settings *model.Setting, roleMapConns map[model.Role][]model.Connection) *Game {
	id := ulid.Make().String()
	agents := util.CreateAgentsWithRole(roleMapConns)
	gameStatus := model.NewInitializeGameStatus(agents)
	gameStatuses := make(map[int]*model.GameStatus)
	gameStatuses[0] = &gameStatus
	slog.Info("ゲームを作成しました", "id", id)
	return &Game{
		config:            config,
		ID:                id,
		setting:           settings,
		Agents:            agents,
		currentDay:        0,
		gameStatuses:      gameStatuses,
		lastTalkIdxMap:    make(map[*model.Agent]int),
		lastWhisperIdxMap: make(map[*model.Agent]int),
		IsFinished:        false,
	}
}

func (g *Game) SetJSONLogger(jsonLogger *service.JSONLogger) {
	g.jsonLogger = jsonLogger
}

func (g *Game) SetGameLogger(gameLogger *service.GameLogger) {
	g.gameLogger = gameLogger
}

func (g *Game) SetRealtimeBroadcaster(realtimeBroadcaster *service.RealtimeBroadcaster) {
	g.realtimeBroadcaster = realtimeBroadcaster
}

func (g *Game) Start() model.Team {
	slog.Info("ゲームを開始します", "id", g.ID)
	if g.jsonLogger != nil {
		g.jsonLogger.TrackStartGame(g.ID, g.Agents)
	}
	if g.gameLogger != nil {
		g.gameLogger.TrackStartGame(g.ID, g.Agents)
	}
	g.requestToEveryone(model.R_INITIALIZE)
	var winSide model.Team = model.T_NONE
	for winSide == model.T_NONE && util.CalcHasErrorAgents(g.Agents) < int(float64(len(g.Agents))*g.config.Game.MaxContinueErrorRatio) {
		g.progressDay()
		g.progressNight()
		gameStatus := g.getCurrentGameStatus().NextDay()
		g.gameStatuses[g.currentDay+1] = &gameStatus
		g.currentDay++
		slog.Info("日付が進みました", "id", g.ID, "day", g.currentDay)
		winSide = util.CalcWinSideTeam(gameStatus.StatusMap)
	}
	if winSide == model.T_NONE {
		slog.Warn("エラーが多発したため、ゲームを終了します", "id", g.ID)
	}
	g.requestToEveryone(model.R_FINISH)
	if g.gameLogger != nil {
		for _, agent := range g.Agents {
			g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,status,%d,%s,%s,%s", g.currentDay, agent.Idx, agent.Role.Name, g.getCurrentGameStatus().StatusMap[*agent].String(), agent.Name))
		}
		villagers, werewolves := util.CountAliveTeams(g.getCurrentGameStatus().StatusMap)
		g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,result,%d,%d,%s", g.currentDay, villagers, werewolves, winSide))
	}
	if g.realtimeBroadcaster != nil {
		packet := g.getRealtimeBroadcastPacket()
		packet.IsDay = true
		packet.Event = "終了"
		message := string(winSide)
		packet.Message = &message
		g.realtimeBroadcaster.Broadcast(packet)
	}
	g.closeAllAgents()
	if g.jsonLogger != nil {
		g.jsonLogger.TrackEndGame(g.ID, winSide)
	}
	if g.gameLogger != nil {
		g.gameLogger.TrackEndGame(g.ID)
	}
	slog.Info("ゲームが終了しました", "id", g.ID, "winSide", winSide)
	g.IsFinished = true
	return winSide
}

func (g *Game) progressDay() {
	slog.Info("昼を開始します", "id", g.ID, "day", g.currentDay)
	g.requestToEveryone(model.R_DAILY_INITIALIZE)
	if g.gameLogger != nil {
		for _, agent := range g.Agents {
			g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,status,%d,%s,%s,%s", g.currentDay, agent.Idx, agent.Role.Name, g.getCurrentGameStatus().StatusMap[*agent].String(), agent.Name))
		}
	}
	if g.setting.TalkOnFirstDay && g.currentDay == 0 {
		g.doWhisper()
	}
	g.doTalk()
	slog.Info("昼を終了します", "id", g.ID, "day", g.currentDay)
}

func (g *Game) progressNight() {
	slog.Info("夜を開始します", "id", g.ID, "day", g.currentDay)
	g.requestToEveryone(model.R_DAILY_FINISH)
	if g.setting.TalkOnFirstDay && g.currentDay == 0 {
		g.doWhisper()
	}
	if g.currentDay != 0 {
		g.doExecution()
	}
	g.doDivine()
	if g.currentDay != 0 {
		g.doWhisper()
		g.doGuard()
		g.doAttack()
	}
	slog.Info("夜を終了します", "id", g.ID, "day", g.currentDay)
}
