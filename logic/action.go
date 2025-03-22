package logic

import (
	"fmt"
	"log/slog"
	"math/rand"
	"unicode/utf8"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/aiwolfdial/aiwolf-nlp-server/util"
)

func (g *Game) doExecution() {
	slog.Info("追放フェーズを開始します", "id", g.ID, "day", g.currentDay)
	var executed *model.Agent
	candidates := make([]model.Agent, 0)
	for i := 0; i < g.setting.Vote.MaxCount; i++ {
		g.executeVote()
		candidates = g.getVotedCandidates(g.getCurrentGameStatus().Votes)
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
		g.getCurrentGameStatus().StatusMap[*executed] = model.S_DEAD
		g.getCurrentGameStatus().ExecutedAgent = executed
		if g.gameLogger != nil {
			g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,execute,%d,%s", g.currentDay, executed.Idx, executed.Role.Name))
		}
		if g.realtimeBroadcaster != nil {
			packet := g.getRealtimeBroadcastPacket()
			packet.IsDay = false
			packet.Event = "追放"
			packet.ToIdx = &executed.Idx
			g.realtimeBroadcaster.Broadcast(packet)
		}
		slog.Info("追放結果を設定しました", "id", g.ID, "agent", executed.String())

		g.getCurrentGameStatus().MediumResult = &model.Judge{
			Day:    g.getCurrentGameStatus().Day,
			Agent:  *executed,
			Target: *executed,
			Result: executed.Role.Species,
		}
		slog.Info("霊能結果を設定しました", "id", g.ID, "target", executed.String(), "result", executed.Role.Species)
	} else {
		if g.realtimeBroadcaster != nil {
			packet := g.getRealtimeBroadcastPacket()
			packet.IsDay = false
			packet.Event = "追放"
			g.realtimeBroadcaster.Broadcast(packet)
		}
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
		for i := 0; i < g.setting.AttackVote.MaxCount; i++ {
			g.executeAttackVote()
			candidates = g.getAttackVotedCandidates(g.getCurrentGameStatus().AttackVotes)
			if len(candidates) == 1 {
				attacked = &candidates[0]
				break
			}
		}
		if attacked == nil && !g.setting.AttackVote.AllowNoTarget && len(candidates) > 0 {
			rand := util.SelectRandomAgent(candidates)
			attacked = &rand
		}

		if attacked != nil && !g.isGuarded(attacked) {
			g.getCurrentGameStatus().StatusMap[*attacked] = model.S_DEAD
			g.getCurrentGameStatus().AttackedAgent = attacked
			if g.gameLogger != nil {
				g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,attack,%d,true", g.currentDay, attacked.Idx))
			}
			if g.realtimeBroadcaster != nil {
				packet := g.getRealtimeBroadcastPacket()
				packet.IsDay = false
				packet.Event = "襲撃"
				packet.ToIdx = &attacked.Idx
				g.realtimeBroadcaster.Broadcast(packet)
			}
			slog.Info("襲撃結果を設定しました", "id", g.ID, "agent", attacked.String())
		} else if attacked != nil {
			if g.gameLogger != nil {
				g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,attack,%d,false", g.currentDay, attacked.Idx))
			}
			if g.realtimeBroadcaster != nil {
				packet := g.getRealtimeBroadcastPacket()
				packet.IsDay = false
				packet.Event = "襲撃"
				idx := -1
				packet.FromIdx = &idx
				packet.ToIdx = &attacked.Idx
				g.realtimeBroadcaster.Broadcast(packet)
			}
			slog.Info("護衛されたため、襲撃結果を設定しません", "id", g.ID, "agent", attacked.String())
		} else {
			if g.gameLogger != nil {
				g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,attack,-1,true", g.currentDay))
			}
			if g.realtimeBroadcaster != nil {
				packet := g.getRealtimeBroadcastPacket()
				packet.IsDay = false
				packet.Event = "襲撃"
				g.realtimeBroadcaster.Broadcast(packet)
			}
			slog.Info("襲撃対象がいないため、襲撃結果を設定しません", "id", g.ID)
		}
	}
	slog.Info("襲撃フェーズを終了します", "id", g.ID, "day", g.currentDay)
}

func (g *Game) isGuarded(attacked *model.Agent) bool {
	if g.getCurrentGameStatus().Guard == nil {
		return false
	}
	return g.getCurrentGameStatus().Guard.Target == *attacked && g.isAlive(&g.getCurrentGameStatus().Guard.Agent)
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
	g.getCurrentGameStatus().DivineResult = &model.Judge{
		Day:    g.getCurrentGameStatus().Day,
		Agent:  *agent,
		Target: *target,
		Result: target.Role.Species,
	}
	if g.gameLogger != nil {
		g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,divine,%d,%d,%s", g.currentDay, agent.Idx, target.Idx, target.Role.Species))
	}
	if g.realtimeBroadcaster != nil {
		packet := g.getRealtimeBroadcastPacket()
		packet.IsDay = false
		packet.Event = "占い"
		packet.FromIdx = &agent.Idx
		packet.ToIdx = &target.Idx
		g.realtimeBroadcaster.Broadcast(packet)
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
	g.getCurrentGameStatus().Guard = &model.Guard{
		Day:    g.getCurrentGameStatus().Day,
		Agent:  *agent,
		Target: *target,
	}
	if g.gameLogger != nil {
		g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,guard,%d,%d,%s", g.currentDay, agent.Idx, target.Idx, target.Role.Name))
	}
	if g.realtimeBroadcaster != nil {
		packet := g.getRealtimeBroadcastPacket()
		packet.IsDay = false
		packet.Event = "護衛"
		packet.FromIdx = &agent.Idx
		packet.ToIdx = &target.Idx
		g.realtimeBroadcaster.Broadcast(packet)
	}
	slog.Info("護衛対象を設定しました", "id", g.ID, "target", target.String())
}

func (g *Game) executeVote() {
	slog.Info("投票アクションを開始します", "id", g.ID, "day", g.currentDay)
	g.getCurrentGameStatus().Votes = g.collectVotes(model.R_VOTE, g.getAliveAgents())
}

func (g *Game) executeAttackVote() {
	slog.Info("襲撃投票アクションを開始します", "id", g.ID, "day", g.currentDay)
	g.getCurrentGameStatus().AttackVotes = g.collectVotes(model.R_ATTACK, g.getAliveWerewolves())
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
			Day:    g.getCurrentGameStatus().Day,
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

		if g.realtimeBroadcaster != nil {
			if request == model.R_VOTE {
				packet := g.getRealtimeBroadcastPacket()
				packet.IsDay = false
				packet.Event = "投票"
				packet.FromIdx = &agent.Idx
				packet.ToIdx = &target.Idx
				g.realtimeBroadcaster.Broadcast(packet)
			} else {
				packet := g.getRealtimeBroadcastPacket()
				packet.IsDay = false
				packet.Event = "襲撃投票"
				packet.FromIdx = &agent.Idx
				packet.ToIdx = &target.Idx
				g.realtimeBroadcaster.Broadcast(packet)
			}
		}
		slog.Info("投票を受信しました", "id", g.ID, "agent", agent.String(), "target", target.String())
	}
	return votes
}

func (g *Game) doWhisper() {
	slog.Info("囁きフェーズを開始します", "id", g.ID, "day", g.currentDay)
	g.conductCommunication(model.R_WHISPER)
}

func (g *Game) doTalk() {
	slog.Info("トークフェーズを開始します", "id", g.ID, "day", g.currentDay)
	g.conductCommunication(model.R_TALK)
}

func (g *Game) conductCommunication(request model.Request) {
	var agents []*model.Agent
	var maxCountPerAgent, maxLengthPerTalk, maxLengthPerAgent, maxSkip int
	var talkList *[]model.Talk
	var maxCountPerDay, baseLength int
	switch request {
	case model.R_TALK:
		agents = g.getAliveAgents()
		maxCountPerAgent = g.setting.Talk.MaxCount.PerAgent
		maxLengthPerTalk = g.setting.Talk.MaxLength.PerTalk
		maxLengthPerAgent = g.setting.Talk.MaxLength.PerAgent
		maxSkip = g.setting.Talk.MaxSkip
		talkList = &g.getCurrentGameStatus().Talks
		maxCountPerDay = g.setting.Talk.MaxCount.PerDay
		baseLength = g.setting.Talk.MaxLength.BaseLength
	case model.R_WHISPER:
		agents = g.getAliveWerewolves()
		maxCountPerAgent = g.setting.Whisper.MaxCount.PerAgent
		maxLengthPerTalk = g.setting.Whisper.MaxLength.PerTalk
		maxLengthPerAgent = g.setting.Whisper.MaxLength.PerAgent
		maxSkip = g.setting.Whisper.MaxSkip
		talkList = &g.getCurrentGameStatus().Whispers
		maxCountPerDay = g.setting.Whisper.MaxCount.PerDay
		baseLength = g.setting.Whisper.MaxLength.BaseLength
	default:
		return
	}
	if len(agents) < 2 {
		slog.Warn("エージェント数が2未満のため、通信を行いません", "id", g.ID, "agentNum", len(agents))
		return
	}
	remainCountMap := make(map[model.Agent]int)
	remainLengthMap := make(map[model.Agent]int)
	remainSkipMap := make(map[model.Agent]int)
	for _, agent := range agents {
		remainCountMap[*agent] = maxCountPerAgent
		remainLengthMap[*agent] = maxLengthPerAgent
		remainSkipMap[*agent] = maxSkip
	}
	g.getCurrentGameStatus().RemainCountMap = &remainCountMap
	g.getCurrentGameStatus().RemainLengthMap = &remainLengthMap
	g.getCurrentGameStatus().RemainSkipMap = &remainSkipMap

	rand.Shuffle(len(agents), func(i, j int) {
		agents[i], agents[j] = agents[j], agents[i]
	})

	idx := 0
	for i := range maxCountPerDay {
		cnt := false
		for _, agent := range agents {
			if remainCountMap[*agent] <= 0 {
				continue
			}
			text := g.getTalkWhisperText(agent, request)
			remainCountMap[*agent]--
			if text == model.T_SKIP {
				if remainSkipMap[*agent] <= 0 {
					text = model.T_OVER
					slog.Warn("スキップ回数が上限に達したため、発言をオーバーに置換しました", "id", g.ID, "agent", agent.String())
				} else {
					remainSkipMap[*agent]--
					slog.Info("発言をスキップしました", "id", g.ID, "agent", agent.String())
				}
			} else if text == model.T_FORCE_SKIP {
				text = model.T_SKIP
				slog.Warn("強制スキップが指定されたため、発言をスキップに置換しました", "id", g.ID, "agent", agent.String())
			}
			if text != model.T_OVER && text != model.T_SKIP {
				remainSkipMap[*agent] = maxSkip
				slog.Info("発言がオーバーもしくはスキップではないため、スキップ回数をリセットしました", "id", g.ID, "agent", agent.String())
			}
			if maxLengthPerAgent != -1 {
				length := utf8.RuneCountInString(text)
				length -= baseLength
				if remainLengthMap[*agent] == 0 {
					text = model.T_OVER
					slog.Warn("残り文字数が0のため、発言をオーバーに置換しました", "id", g.ID, "agent", agent.String())
				} else if length > remainLengthMap[*agent] {
					text = string([]rune(text)[:remainLengthMap[*agent]])
					remainLengthMap[*agent] = 0
					slog.Warn("発言が最大文字数を超えたため、切り捨てました", "id", g.ID, "agent", agent.String())
				} else {
					remainLengthMap[*agent] -= length
				}
			}
			if maxLengthPerTalk != -1 {
				if utf8.RuneCountInString(text) > maxLengthPerTalk {
					text = string([]rune(text)[:maxLengthPerTalk])
					slog.Warn("発言が最大文字数を超えたため、切り捨てました", "id", g.ID, "agent", agent.String())
				}
			}
			talk := model.Talk{
				Idx:   idx,
				Day:   g.getCurrentGameStatus().Day,
				Turn:  i,
				Agent: *agent,
				Text:  text,
			}
			idx++
			*talkList = append(*talkList, talk)
			if text != model.T_OVER {
				cnt = true
			} else {
				remainCountMap[*agent] = 0
				slog.Info("発言がオーバーであるため、残り発言回数を0にしました", "id", g.ID, "agent", agent.String())
			}
			if g.gameLogger != nil {
				if request == model.R_TALK {
					g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,talk,%d,%d,%d,%s", g.currentDay, talk.Idx, talk.Turn, talk.Agent.Idx, talk.Text))
				} else {
					g.gameLogger.AppendLog(g.ID, fmt.Sprintf("%d,whisper,%d,%d,%d,%s", g.currentDay, talk.Idx, talk.Turn, talk.Agent.Idx, talk.Text))
				}
			}
			if g.realtimeBroadcaster != nil {
				if request == model.R_TALK {
					packet := g.getRealtimeBroadcastPacket()
					packet.IsDay = true
					packet.Event = "トーク"
					packet.Message = &talk.Text
					packet.BubbleIdx = &agent.Idx
					g.realtimeBroadcaster.Broadcast(packet)
				} else {
					packet := g.getRealtimeBroadcastPacket()
					packet.IsDay = true
					packet.Event = "囁き"
					packet.Message = &talk.Text
					packet.BubbleIdx = &agent.Idx
					g.realtimeBroadcaster.Broadcast(packet)
				}
			}
			slog.Info("発言を受信しました", "id", g.ID, "agent", agent.String(), "text", text, "count", remainCountMap[*agent], "length", remainLengthMap[*agent], "skip", remainSkipMap[*agent])
		}
		if !cnt {
			break
		}
	}

	g.getCurrentGameStatus().RemainCountMap = nil
	g.getCurrentGameStatus().RemainLengthMap = nil
	g.getCurrentGameStatus().RemainSkipMap = nil
}

func (g *Game) getTalkWhisperText(agent *model.Agent, request model.Request) string {
	text, err := g.requestToAgent(agent, request)
	if text == model.T_FORCE_SKIP {
		text = model.T_SKIP
		slog.Warn("クライアントから強制スキップが指定されたため、発言をスキップに置換しました", "id", g.ID, "agent", agent.String())
	}
	if err != nil {
		text = model.T_FORCE_SKIP
		slog.Warn("リクエストの送受信に失敗したため、発言をスキップに置換しました", "id", g.ID, "agent", agent.String())
	}
	return text
}
