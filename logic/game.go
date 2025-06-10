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
	agents                       []*model.Agent
	winSide                      model.Team
	isFinished                   bool
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
	if config.CustomProfile.Enable {
		if config.CustomProfile.DynamicProfile.Enable {
			profiles, err := util.GenerateProfiles(config.CustomProfile.DynamicProfile.Prompt, config.CustomProfile.DynamicProfile.Avatars, config.Game.AgentCount, config.CustomProfile.DynamicProfile.Attempts)
			if err != nil {
				slog.Error("プロフィールの生成に失敗したため、カスタムプロフィールを使用します", "error", err)
				agents = util.CreateAgentsWithProfiles(conns, settings.RoleNumMap, config.CustomProfile.Profiles)
			} else {
				agents = util.CreateAgentsWithProfiles(conns, settings.RoleNumMap, profiles)
			}
		} else {
			agents = util.CreateAgentsWithProfiles(conns, settings.RoleNumMap, config.CustomProfile.Profiles)
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
		agents:            agents,
		winSide:           model.T_NONE,
		isFinished:        false,
		config:            config,
		setting:           settings,
		currentDay:        0,
		isDaytime:         true,
		gameStatuses:      gameStatuses,
		lastTalkIdxMap:    make(map[*model.Agent]int),
		lastWhisperIdxMap: make(map[*model.Agent]int),
	}
}

func NewGameWithRole(config *model.Config, settings *model.Setting, roleMapConns map[model.Role][]model.Connection) *Game {
	id := ulid.Make().String()
	var agents []*model.Agent
	if config.CustomProfile.Enable {
		if config.CustomProfile.DynamicProfile.Enable {
			profiles, err := util.GenerateProfiles(config.CustomProfile.DynamicProfile.Prompt, config.CustomProfile.DynamicProfile.Avatars, config.Game.AgentCount, config.CustomProfile.DynamicProfile.Attempts)
			if err != nil {
				slog.Error("プロフィールの生成に失敗したため、カスタムプロフィールを使用します", "error", err)
				agents = util.CreateAgentsWithRoleAndProfile(roleMapConns, config.CustomProfile.Profiles)
			} else {
				agents = util.CreateAgentsWithRoleAndProfile(roleMapConns, profiles)
			}
		} else {
			agents = util.CreateAgentsWithRoleAndProfile(roleMapConns, config.CustomProfile.Profiles)
		}
	} else {
		agents = util.CreateAgentsWithRole(roleMapConns)
	}
	gameStatus := model.NewInitializeGameStatus(agents)
	gameStatuses := make(map[int]*model.GameStatus)
	gameStatuses[0] = &gameStatus
	slog.Info("ゲームを作成しました", "id", id)
	return &Game{
		ID:                id,
		agents:            agents,
		winSide:           model.T_NONE,
		isFinished:        false,
		config:            config,
		setting:           settings,
		currentDay:        0,
		isDaytime:         true,
		gameStatuses:      gameStatuses,
		lastTalkIdxMap:    make(map[*model.Agent]int),
		lastWhisperIdxMap: make(map[*model.Agent]int),
	}
}

func (g *Game) Start() model.Team {
	slog.Info("ゲームを開始します", "id", g.ID)
	if g.JsonLogger != nil {
		g.JsonLogger.TrackStartGame(g.ID, g.agents)
	}
	if g.GameLogger != nil {
		g.GameLogger.TrackStartGame(g.ID, g.agents)
	}
	if g.RealtimeBroadcaster != nil {
		g.RealtimeBroadcaster.TrackStartGame(g.ID, g.agents)
	}
	if g.TTSBroadcaster != nil {
		g.TTSBroadcaster.CreateStream(g.ID)
	}
	if g.RealtimeBroadcaster != nil {
		packet := g.getRealtimeBroadcastPacket()
		packet.Event = "開始"
		message := "ゲームが開始されました"
		packet.Message = &message
		g.RealtimeBroadcaster.Broadcast(packet)
	}
	if g.TTSBroadcaster != nil {
		g.TTSBroadcaster.BroadcastText(g.ID, "ゲームが開始されました", 23)
	}
	g.requestToEveryone(model.R_INITIALIZE)
	for {
		g.progressDay()
		g.progressNight()
		gameStatus := g.getCurrentGameStatus().NextDay()
		g.gameStatuses[g.currentDay+1] = &gameStatus
		g.currentDay++
		slog.Info("日付が進みました", "id", g.ID, "day", g.currentDay)
		if g.config.Game.MaxDay > 0 && g.currentDay >= g.config.Game.MaxDay+1 {
			slog.Info("最大日数に達したため、ゲームを終了します", "id", g.ID, "day", g.currentDay)
			break
		}
		if g.shouldFinish() {
			break
		}
	}
	g.requestToEveryone(model.R_FINISH)
	if g.GameLogger != nil {
		for _, agent := range g.agents {
			g.GameLogger.AppendLog(g.ID, fmt.Sprintf("%d,status,%d,%s,%s,%s,%s", g.currentDay, agent.Idx, agent.Role.Name, g.getCurrentGameStatus().StatusMap[*agent].String(), agent.OriginalName, agent.GameName))
		}
		villagers, werewolves := util.CountAliveTeams(g.getCurrentGameStatus().StatusMap)
		g.GameLogger.AppendLog(g.ID, fmt.Sprintf("%d,result,%d,%d,%s", g.currentDay, villagers, werewolves, g.winSide))
	}
	if g.RealtimeBroadcaster != nil {
		packet := g.getRealtimeBroadcastPacket()
		packet.Event = "終了"
		message := string(g.winSide)
		packet.Message = &message
		g.RealtimeBroadcaster.Broadcast(packet)
	}
	if g.TTSBroadcaster != nil {
		g.TTSBroadcaster.BroadcastText(g.ID, "ゲームが終了しました", 23)
	}
	g.closeAllAgents()
	if g.JsonLogger != nil {
		g.JsonLogger.TrackEndGame(g.ID, g.winSide)
	}
	if g.GameLogger != nil {
		g.GameLogger.TrackEndGame(g.ID)
	}
	if g.RealtimeBroadcaster != nil {
		g.RealtimeBroadcaster.TrackEndGame(g.ID)
	}
	slog.Info("ゲームが終了しました", "id", g.ID, "winSide", g.winSide)
	g.isFinished = true
	return g.winSide
}

func (g *Game) shouldFinish() bool {
	if util.CalcHasErrorAgents(g.agents) >= int(float64(len(g.agents))*g.config.Server.MaxContinueErrorRatio) {
		slog.Warn("エラーが多発したため、ゲームを終了します", "id", g.ID)
		return true
	}
	g.winSide = util.CalcWinSideTeam(g.getCurrentGameStatus().StatusMap)
	if g.winSide != model.T_NONE {
		slog.Info("勝利チームが決定したため、ゲームを終了します", "id", g.ID)
		return true
	}
	return false
}

func (g *Game) progressDay() {
	slog.Info("昼セクションを開始します", "id", g.ID, "day", g.currentDay)
	g.isDaytime = true
	g.requestToEveryone(model.R_DAILY_INITIALIZE)
	if g.GameLogger != nil {
		for _, agent := range g.agents {
			g.GameLogger.AppendLog(g.ID, fmt.Sprintf("%d,status,%d,%s,%s,%s,%s", g.currentDay, agent.Idx, agent.Role.Name, g.getCurrentGameStatus().StatusMap[*agent].String(), agent.OriginalName, agent.GameName))
		}
	}

	for _, phase := range g.config.Logic.DayPhases {
		if phase.OnlyDay != nil && *phase.OnlyDay != g.currentDay {
			slog.Info("実行対象の日ではないため、フェーズをスキップします", "id", g.ID, "day", g.currentDay, "phase", phase.Name)
			continue
		}
		if phase.ExceptDay != nil && *phase.ExceptDay == g.currentDay {
			slog.Info("除外対象の日であるため、フェーズをスキップします", "id", g.ID, "day", g.currentDay, "phase", phase.Name)
			continue
		}
		slog.Info("昼セクションのフェーズを開始します", "id", g.ID, "day", g.currentDay, "phase", phase.Name)
		g.executePhase(phase.Actions)
		if g.shouldFinish() {
			return
		}
	}

	slog.Info("昼セクションを終了します", "id", g.ID, "day", g.currentDay)
}

func (g *Game) progressNight() {
	slog.Info("夜セクションを開始します", "id", g.ID, "day", g.currentDay)
	g.isDaytime = false
	g.requestToEveryone(model.R_DAILY_FINISH)

	for _, phase := range g.config.Logic.NightPhases {
		if phase.OnlyDay != nil && *phase.OnlyDay != g.currentDay {
			slog.Info("実行対象の日ではないため、フェーズをスキップします", "id", g.ID, "day", g.currentDay, "phase", phase.Name)
			continue
		}
		if phase.ExceptDay != nil && *phase.ExceptDay == g.currentDay {
			slog.Info("除外対象の日であるため、フェーズをスキップします", "id", g.ID, "day", g.currentDay, "phase", phase.Name)
			continue
		}
		slog.Info("夜セクションのフェーズを実行します", "id", g.ID, "day", g.currentDay, "phase", phase.Name)
		g.executePhase(phase.Actions)
		if g.shouldFinish() {
			return
		}
	}

	slog.Info("夜セクションを終了します", "id", g.ID, "day", g.currentDay)
}

func (g *Game) executePhase(actions []string) {
	for _, action := range actions {
		switch action {
		case "talk":
			g.doTalk()
		case "whisper":
			g.doWhisper()
		case "execution":
			g.doExecution()
		case "divine":
			g.doDivine()
		case "guard":
			g.doGuard()
		case "attack":
			g.doAttack()
		default:
			slog.Warn("不明なアクションです", "action", action)
		}
	}
}
