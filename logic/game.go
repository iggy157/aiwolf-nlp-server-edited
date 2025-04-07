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
	isDaytime                    bool
	gameStatuses                 map[int]*model.GameStatus
	lastTalkIdxMap               map[*model.Agent]int
	lastWhisperIdxMap            map[*model.Agent]int
	JsonLogger                   *service.JSONLogger
	GameLogger                   *service.GameLogger
	RealtimeBroadcaster          *service.RealtimeBroadcaster
	TTSBroadcaster               *service.TTSBroadcaster
	realtimeBroadcasterPacketIdx int
}

func NewGame(config *model.Config, settings *model.Setting, conns []model.Connection) *Game {
	id := ulid.Make().String()
	var agents []*model.Agent
	if config.Game.CustomProfile.Enable {
		if config.Game.CustomProfile.DynamicProfile.Enable {
			profiles, err := util.GenerateProfiles(config.Game.CustomProfile.DynamicProfile.Prompt, config.Game.CustomProfile.DynamicProfile.Avatars, config.Game.AgentCount, config.Game.CustomProfile.DynamicProfile.Attempts)
			if err != nil {
				slog.Error("プロフィールの生成に失敗したため、カスタムプロフィールを使用します", "error", err)
				agents = util.CreateAgentsWithProfiles(conns, settings.RoleNumMap, config.Game.CustomProfile.Profiles)
			} else {
				agents = util.CreateAgentsWithProfiles(conns, settings.RoleNumMap, profiles)
			}
		} else {
			agents = util.CreateAgentsWithProfiles(conns, settings.RoleNumMap, config.Game.CustomProfile.Profiles)
		}
	} else {
		agents = util.CreateAgents(conns, settings.RoleNumMap)
	}
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
	var agents []*model.Agent
	if config.Game.CustomProfile.Enable {
		if config.Game.CustomProfile.DynamicProfile.Enable {
			profiles, err := util.GenerateProfiles(config.Game.CustomProfile.DynamicProfile.Prompt, config.Game.CustomProfile.DynamicProfile.Avatars, config.Game.AgentCount, config.Game.CustomProfile.DynamicProfile.Attempts)
			if err != nil {
				slog.Error("プロフィールの生成に失敗したため、カスタムプロフィールを使用します", "error", err)
				agents = util.CreateAgentsWithRoleAndProfile(roleMapConns, config.Game.CustomProfile.Profiles)
			} else {
				agents = util.CreateAgentsWithRoleAndProfile(roleMapConns, profiles)
			}
		} else {
			agents = util.CreateAgentsWithRoleAndProfile(roleMapConns, config.Game.CustomProfile.Profiles)
		}
	} else {
		agents = util.CreateAgentsWithRole(roleMapConns)
	}
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

func (g *Game) Start() model.Team {
	slog.Info("ゲームを開始します", "id", g.ID)
	if g.JsonLogger != nil {
		g.JsonLogger.TrackStartGame(g.ID, g.Agents)
	}
	if g.GameLogger != nil {
		g.GameLogger.TrackStartGame(g.ID, g.Agents)
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
	if g.GameLogger != nil {
		for _, agent := range g.Agents {
			g.GameLogger.AppendLog(g.ID, fmt.Sprintf("%d,status,%d,%s,%s,%s", g.currentDay, agent.Idx, agent.Role.Name, g.getCurrentGameStatus().StatusMap[*agent].String(), agent.OriginalName))
		}
		villagers, werewolves := util.CountAliveTeams(g.getCurrentGameStatus().StatusMap)
		g.GameLogger.AppendLog(g.ID, fmt.Sprintf("%d,result,%d,%d,%s", g.currentDay, villagers, werewolves, winSide))
	}
	if g.RealtimeBroadcaster != nil {
		packet := g.getRealtimeBroadcastPacket()
		packet.Event = "終了"
		message := string(winSide)
		packet.Message = &message
		g.RealtimeBroadcaster.Broadcast(packet)
	}
	g.closeAllAgents()
	if g.JsonLogger != nil {
		g.JsonLogger.TrackEndGame(g.ID, winSide)
	}
	if g.GameLogger != nil {
		g.GameLogger.TrackEndGame(g.ID)
	}
	slog.Info("ゲームが終了しました", "id", g.ID, "winSide", winSide)
	g.IsFinished = true
	return winSide
}

func (g *Game) progressDay() {
	slog.Info("昼セクションを開始します", "id", g.ID, "day", g.currentDay)
	g.isDaytime = true
	g.requestToEveryone(model.R_DAILY_INITIALIZE)
	if g.GameLogger != nil {
		for _, agent := range g.Agents {
			g.GameLogger.AppendLog(g.ID, fmt.Sprintf("%d,status,%d,%s,%s,%s", g.currentDay, agent.Idx, agent.Role.Name, g.getCurrentGameStatus().StatusMap[*agent].String(), agent.OriginalName))
		}
	}
	if g.setting.TalkOnFirstDay && g.currentDay == 0 {
		g.doWhisper()
	}
	g.doTalk()
	slog.Info("昼セクションを終了します", "id", g.ID, "day", g.currentDay)
}

func (g *Game) progressNight() {
	slog.Info("夜セクションを開始します", "id", g.ID, "day", g.currentDay)
	g.isDaytime = false
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
	slog.Info("夜セクションを終了します", "id", g.ID, "day", g.currentDay)
}
