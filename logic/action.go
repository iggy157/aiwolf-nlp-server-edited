package logic

import (
	"fmt"
	"log/slog"
	"math/rand"

	"github.com/kano-lab/aiwolf-nlp-server/model"
	"github.com/kano-lab/aiwolf-nlp-server/util"
)

func (g *Game) doExecution() {
	slog.Info("追放フェーズを開始します", "id", g.ID, "day", g.currentDay)
	var executed *model.Agent
	candidates := make([]model.Agent, 0)
	for i := 0; i < g.settings.MaxRevote; i++ {
		g.executeVote()
		candidates = g.getVotedCandidates(g.gameStatuses[g.currentDay].Votes)
		if len(candidates) == 1 {
			executed = &candidates[0]
			break
		}
	}
	if executed == nil && len(candidates) > 0 {
		rand := util.SelectRandomAgent(candidates)
		executed = &rand
	}
	if executed != nil {
		g.gameStatuses[g.currentDay].StatusMap[*executed] = model.S_DEAD
		g.gameStatuses[g.currentDay].ExecutedAgent = executed
		slog.Info("追放結果を設定しました", "id", g.ID, "agent", executed.String())

		g.gameStatuses[g.currentDay].MediumResult = &model.Judge{
			Day:    g.gameStatuses[g.currentDay].Day,
			Agent:  *executed,
			Target: *executed,
			Result: executed.Role.Species,
		}
		if g.gameLogger != nil {
			g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,execute,%d,%s", g.currentDay, executed.Idx, executed.Role.Name))
		}
		slog.Info("霊能結果を設定しました", "id", g.ID, "target", executed.String(), "result", executed.Role.Species)
	} else {
		slog.Warn("追放対象がいないため、追放結果を設定しません", "id", g.ID)
	}
	slog.Info("追放フェーズを終了します", "id", g.ID, "day", g.currentDay)
	if g.realtimeBroadcaster != nil {
	}
}

func (g *Game) doAttack() {
	slog.Info("襲撃フェーズを開始します", "id", g.ID, "day", g.currentDay)
	var attacked *model.Agent
	werewolfs := g.getAliveWerewolves()
	if len(werewolfs) > 0 {
		candidates := make([]model.Agent, 0)
		for i := 0; i < g.settings.MaxAttackRevote; i++ {
			g.executeAttackVote()
			candidates = g.getAttackVotedCandidates(g.gameStatuses[g.currentDay].AttackVotes)
			if len(candidates) == 1 {
				attacked = &candidates[0]
				break
			}
		}
		if attacked == nil && !g.settings.IsEnableNoAttack && len(candidates) > 0 {
			rand := util.SelectRandomAgent(candidates)
			attacked = &rand
		}

		if attacked != nil && !g.isGuarded(attacked) {
			g.gameStatuses[g.currentDay].StatusMap[*attacked] = model.S_DEAD
			g.gameStatuses[g.currentDay].AttackedAgent = attacked
			if g.gameLogger != nil {
				g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,attack,%d,true", g.currentDay, attacked.Idx))
			}
			slog.Info("襲撃結果を設定しました", "id", g.ID, "agent", attacked.String())
		} else if attacked != nil {
			if g.gameLogger != nil {
				g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,attack,%d,false", g.currentDay, attacked.Idx))
			}
			slog.Info("護衛されたため、襲撃結果を設定しません", "id", g.ID, "agent", attacked.String())
		} else {
			if g.gameLogger != nil {
				g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,attack,-1,true", g.currentDay))
			}
			slog.Info("襲撃対象がいないため、襲撃結果を設定しません", "id", g.ID)
		}
	}
	slog.Info("襲撃フェーズを終了します", "id", g.ID, "day", g.currentDay)
}

func (g *Game) isGuarded(attacked *model.Agent) bool {
	if g.gameStatuses[g.currentDay].Guard == nil {
		return false
	}
	return g.gameStatuses[g.currentDay].Guard.Target == *attacked && g.isAlive(&g.gameStatuses[g.currentDay].Guard.Agent)
}

func (g *Game) doDivine() {
	slog.Info("占いフェーズを開始します", "id", g.ID, "day", g.currentDay)
	for _, agent := range g.getAliveAgents() {
		if agent.Role == model.R_SEER {
			g.conductDivination(agent)
			break
		}
	}
	slog.Info("占いフェーズを終了します", "id", g.ID, "day", g.currentDay)
}

func (g *Game) conductDivination(agent *model.Agent) {
	slog.Info("占いアクションを開始します", "id", g.ID, "agent", agent.String())
	target, err := g.findTargetByRequest(agent, model.R_DIVINE)
	if err != nil {
		slog.Warn("占い対象が見つからなかったため、占い結果を設定しません", "id", g.ID)
		return
	}
	if !g.isAlive(target) {
		slog.Warn("占い対象が死亡しているため、占い結果を設定しません", "id", g.ID, "target", target.String())
		return
	}
	if agent == target {
		slog.Warn("占い対象が自分自身であるため、占い結果を設定しません", "id", g.ID, "target", target.String())
		return
	}
	g.gameStatuses[g.currentDay].DivineResult = &model.Judge{
		Day:    g.gameStatuses[g.currentDay].Day,
		Agent:  *agent,
		Target: *target,
		Result: target.Role.Species,
	}
	if g.gameLogger != nil {
		g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,divine,%d,%d,%s", g.currentDay, agent.Idx, target.Idx, target.Role.Species))
	}
	slog.Info("占い結果を設定しました", "id", g.ID, "target", target.String(), "result", target.Role.Species)
}

func (g *Game) doGuard() {
	slog.Info("護衛フェーズを開始します", "id", g.ID, "day", g.currentDay)
	for _, agent := range g.getAliveAgents() {
		if agent.Role == model.R_BODYGUARD {
			g.conductGuard(agent)
			break
		}
	}
}

func (g *Game) conductGuard(agent *model.Agent) {
	slog.Info("護衛アクションを実行します", "id", g.ID, "agent", agent.String())
	target, err := g.findTargetByRequest(agent, model.R_GUARD)
	if err != nil {
		slog.Warn("護衛対象が見つからなかったため、護衛対象を設定しません", "id", g.ID)
		return
	}
	if !g.isAlive(target) {
		slog.Warn("護衛対象が死亡しているため、護衛対象を設定しません", "id", g.ID, "target", target.String())
		return
	}
	if agent == target {
		slog.Warn("護衛対象が自分自身であるため、護衛対象を設定しません", "id", g.ID, "target", target.String())
		return
	}
	g.gameStatuses[g.currentDay].Guard = &model.Guard{
		Day:    g.gameStatuses[g.currentDay].Day,
		Agent:  *agent,
		Target: *target,
	}
	if g.gameLogger != nil {
		g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,guard,%d,%d,%s", g.currentDay, agent.Idx, target.Idx, target.Role.Name))
	}
	slog.Info("護衛対象を設定しました", "id", g.ID, "target", target.String())
}

func (g *Game) executeVote() {
	slog.Info("投票アクションを開始します", "id", g.ID, "day", g.currentDay)
	g.gameStatuses[g.currentDay].Votes = g.collectVotes(model.R_VOTE, g.getAliveAgents())
}

func (g *Game) executeAttackVote() {
	slog.Info("襲撃投票アクションを開始します", "id", g.ID, "day", g.currentDay)
	g.gameStatuses[g.currentDay].AttackVotes = g.collectVotes(model.R_ATTACK, g.getAliveWerewolves())
}

func (g *Game) collectVotes(request model.Request, agents []*model.Agent) []model.Vote {
	votes := make([]model.Vote, 0)
	if request != model.R_VOTE && request != model.R_ATTACK {
		return votes
	}
	for _, agent := range agents {
		target, err := g.findTargetByRequest(agent, request)
		if err != nil {
			continue
		}
		if !g.isAlive(target) {
			slog.Warn("投票対象が死亡しているため、投票を無視します", "id", g.ID, "agent", agent.String(), "target", target.String())
			continue
		}
		votes = append(votes, model.Vote{
			Day:    g.gameStatuses[g.currentDay].Day,
			Agent:  *agent,
			Target: *target,
		})
		if g.gameLogger != nil {
			if request == model.R_VOTE {
				g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,vote,%d,%d", g.currentDay, agent.Idx, target.Idx))
			} else {
				g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,attackVote,%d,%d", g.currentDay, agent.Idx, target.Idx))
			}
		}
		slog.Info("投票を受信しました", "id", g.ID, "agent", agent.String(), "target", target.String())
	}
	return votes
}

func (g *Game) doWhisper() {
	slog.Info("囁きフェーズを開始します", "id", g.ID, "day", g.currentDay)
	g.gameStatuses[g.currentDay].ResetRemainWhisperMap(g.settings.MaxWhisper)
	g.conductCommunication(model.R_WHISPER)
	g.gameStatuses[g.currentDay].ClearRemainWhisperMap()
}

func (g *Game) doTalk() {
	slog.Info("トークフェーズを開始します", "id", g.ID, "day", g.currentDay)
	g.gameStatuses[g.currentDay].ResetRemainTalkMap(g.settings.MaxTalk)
	g.conductCommunication(model.R_TALK)
	g.gameStatuses[g.currentDay].ClearRemainTalkMap()
}

func (g *Game) conductCommunication(request model.Request) {
	var agents []*model.Agent
	var maxTurn int
	var remainMap map[model.Agent]int
	var talkList *[]model.Talk
	switch request {
	case model.R_TALK:
		agents = g.getAliveAgents()
		maxTurn = g.settings.MaxTalkTurn
		remainMap = g.gameStatuses[g.currentDay].RemainTalkMap
		talkList = &g.gameStatuses[g.currentDay].Talks
	case model.R_WHISPER:
		agents = g.getAliveWerewolves()
		maxTurn = g.settings.MaxWhisperTurn
		remainMap = g.gameStatuses[g.currentDay].RemainWhisperMap
		talkList = &g.gameStatuses[g.currentDay].Whispers
	default:
		return
	}

	if len(agents) < 2 {
		slog.Warn("エージェント数が2未満のため、通信を行いません", "id", g.ID, "agentNum", len(agents))
		return
	}

	rand.Shuffle(len(agents), func(i, j int) {
		agents[i], agents[j] = agents[j], agents[i]
	})
	skipMap := make(map[model.Agent]int)
	idx := 0

	for i := 0; i < maxTurn; i++ {
		cnt := false
		for _, agent := range agents {
			if remainMap[*agent] <= 0 {
				continue
			}
			text := g.getTalkWhisperText(agent, request, skipMap, remainMap)
			talk := model.Talk{
				Idx:   idx,
				Day:   g.gameStatuses[g.currentDay].Day,
				Turn:  i,
				Agent: *agent,
				Text:  text,
			}
			idx++
			*talkList = append(*talkList, talk)
			if text != model.T_OVER {
				cnt = true
			} else {
				remainMap[*agent] = 0
				slog.Info("発言がオーバーであるため、残り発言回数を0にしました", "id", g.ID, "agent", agent.String())
			}
			if g.gameLogger != nil {
				if request == model.R_TALK {
					g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,talk,%d,%d,%d,%s", g.currentDay, talk.Idx, talk.Turn, talk.Agent.Idx, talk.Text))
				} else {
					g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,whisper,%d,%d,%d,%s", g.currentDay, talk.Idx, talk.Turn, talk.Agent.Idx, talk.Text))
				}
			}
			slog.Info("発言を受信しました", "id", g.ID, "agent", agent.String(), "text", text, "skip", skipMap[*agent], "remain", remainMap[*agent])
		}
		if !cnt {
			break
		}
	}
}

func (g *Game) getTalkWhisperText(agent *model.Agent, request model.Request, skipMap map[model.Agent]int, remainMap map[model.Agent]int) string {
	text, err := g.requestToAgent(agent, request)
	if text == model.T_FORCE_SKIP {
		text = model.T_SKIP
		slog.Warn("クライアントから強制スキップが指定されたため、発言をスキップに置換しました", "id", g.ID, "agent", agent.String())
	}
	if err != nil {
		text = model.T_FORCE_SKIP
		slog.Warn("リクエストの送受信に失敗したため、発言をスキップに置換しました", "id", g.ID, "agent", agent.String())
	}
	remainMap[*agent]--
	if _, exists := skipMap[*agent]; !exists {
		skipMap[*agent] = 0
	}
	if text == model.T_SKIP {
		skipMap[*agent]++
		if skipMap[*agent] >= g.settings.MaxSkip {
			text = model.T_OVER
			slog.Warn("スキップ回数が上限に達したため、発言をオーバーに置換しました", "id", g.ID, "agent", agent.String())
		} else {
			slog.Info("発言をスキップしました", "id", g.ID, "agent", agent.String())
		}
	} else if text == model.T_FORCE_SKIP {
		text = model.T_SKIP
		slog.Warn("強制スキップが指定されたため、発言をスキップに置換しました", "id", g.ID, "agent", agent.String())
	}
	if text != model.T_OVER && text != model.T_SKIP {
		skipMap[*agent] = 0
		slog.Info("発言がオーバーもしくはスキップではないため、スキップ回数をリセットしました", "id", g.ID, "agent", agent.String())
	}
	return text
}
