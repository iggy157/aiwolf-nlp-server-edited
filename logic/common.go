package logic

import (
	"errors"
	"log/slog"
	"time"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
	"github.com/aiwolfdial/aiwolf-nlp-server/util"
)

func (g *Game) findTargetByRequest(agent *model.Agent, request model.Request) (*model.Agent, error) {
	name, err := g.requestToAgent(agent, request)
	if err != nil {
		return nil, err
	}
	target := util.FindAgentByName(g.Agents, name)
	if target == nil {
		return nil, errors.New("対象エージェントが見つかりません")
	}
	slog.Info("対象エージェントを受信しました", "id", g.ID, "agent", agent.String(), "target", target.String())
	return target, nil
}

func (g *Game) getVotedCandidates(votes []model.Vote) []model.Agent {
	return util.GetCandidates(votes, func(vote model.Vote) bool {
		return true
	})
}

func (g *Game) getAttackVotedCandidates(votes []model.Vote) []model.Agent {
	return util.GetCandidates(votes, func(vote model.Vote) bool {
		return vote.Target.Role.Species != model.S_WEREWOLF
	})
}

func (g *Game) closeAllAgents() {
	for _, agent := range g.Agents {
		agent.Close()
	}
}

func (g *Game) requestToEveryone(request model.Request) {
	for _, agent := range g.Agents {
		g.requestToAgent(agent, request)
	}
}

func (g *Game) buildInfo(agent *model.Agent) model.Info {
	info := model.Info{
		GameID: g.ID,
		Day:    g.currentDay,
		Agent:  agent,
	}
	gameStatus := g.getCurrentGameStatus()
	lastGameStatus := g.gameStatuses[g.currentDay-1]
	if lastGameStatus != nil {
		if lastGameStatus.MediumResult != nil && agent.Role == model.R_MEDIUM {
			info.MediumResult = lastGameStatus.MediumResult
		}
		if lastGameStatus.DivineResult != nil && agent.Role == model.R_SEER {
			info.DivineResult = lastGameStatus.DivineResult
		}
		if lastGameStatus.ExecutedAgent != nil {
			info.ExecutedAgent = lastGameStatus.ExecutedAgent
		}
		if lastGameStatus.AttackedAgent != nil {
			info.AttackedAgent = lastGameStatus.AttackedAgent
		}
		if g.setting.VoteVisibility {
			info.VoteList = lastGameStatus.Votes
		}
		if g.setting.VoteVisibility && agent.Role == model.R_WEREWOLF {
			info.AttackVoteList = lastGameStatus.AttackVotes
		}
	}
	info.TalkList = gameStatus.Talks
	if agent.Role == model.R_WEREWOLF {
		info.WhisperList = gameStatus.Whispers
	}
	info.StatusMap = gameStatus.StatusMap
	roleMap := make(map[model.Agent]model.Role)
	roleMap[*agent] = agent.Role
	if agent.Role == model.R_WEREWOLF {
		for a := range gameStatus.StatusMap {
			if a.Role == model.R_WEREWOLF {
				roleMap[a] = a.Role
			}
		}
	}
	info.RoleMap = roleMap
	if gameStatus.RemainCountMap != nil {
		count := (*gameStatus.RemainCountMap)[*agent]
		info.RemainCount = &count
	}
	if gameStatus.RemainLengthMap != nil {
		if value, exists := (*gameStatus.RemainLengthMap)[*agent]; exists {
			info.RemainLength = &value
		}
	}
	if gameStatus.RemainSkipMap != nil {
		count := (*gameStatus.RemainSkipMap)[*agent]
		info.RemainSkip = &count
	}
	return info
}

func (g *Game) requestToAgent(agent *model.Agent, request model.Request) (string, error) {
	info := g.buildInfo(agent)
	var packet model.Packet
	switch request {
	case model.R_NAME:
		packet = model.Packet{Request: &request}
	case model.R_INITIALIZE, model.R_DAILY_INITIALIZE:
		g.resetLastIdxMaps()
		packet = model.Packet{Request: &request, Info: &info, Setting: g.setting}
	case model.R_VOTE, model.R_DIVINE, model.R_GUARD:
		packet = model.Packet{Request: &request, Info: &info}
	case model.R_DAILY_FINISH, model.R_TALK, model.R_WHISPER, model.R_ATTACK:
		packet = model.Packet{Request: &request, Info: &info}
		talks, whispers := g.minimize(agent, info.TalkList, info.WhisperList)
		if request == model.R_TALK || request == model.R_DAILY_FINISH {
			packet.TalkHistory = &talks
		}
		if request == model.R_WHISPER || request == model.R_ATTACK || (request == model.R_DAILY_FINISH && agent.Role == model.R_WEREWOLF) {
			packet.WhisperHistory = &whispers
		}
	case model.R_FINISH:
		info.RoleMap = util.GetRoleMap(g.Agents)
		packet = model.Packet{Request: &request, Info: &info}
	default:
		return "", errors.New("一致するリクエストがありません")
	}
	if g.jsonLogger != nil {
		g.jsonLogger.TrackStartRequest(g.ID, *agent, packet)
	}
	resp, err := agent.SendPacket(packet, time.Duration(g.setting.Timeout.Action)*time.Millisecond, time.Duration(g.setting.Timeout.Response)*time.Millisecond, g.config.Game.Timeout.Acceptable)
	if g.jsonLogger != nil {
		g.jsonLogger.TrackEndRequest(g.ID, *agent, resp, err)
	}
	return resp, err
}

func (g *Game) resetLastIdxMaps() {
	g.lastTalkIdxMap = make(map[*model.Agent]int)
	g.lastWhisperIdxMap = make(map[*model.Agent]int)
}

func (g *Game) minimize(agent *model.Agent, talks []model.Talk, whispers []model.Talk) ([]model.Talk, []model.Talk) {
	lastTalkIdx := g.lastTalkIdxMap[agent]
	lastWhisperIdx := g.lastWhisperIdxMap[agent]
	g.lastTalkIdxMap[agent] = len(talks)
	g.lastWhisperIdxMap[agent] = len(whispers)
	return talks[lastTalkIdx:], whispers[lastWhisperIdx:]
}

func (g *Game) getCurrentGameStatus() *model.GameStatus {
	return g.gameStatuses[g.currentDay]
}

func (g *Game) getAliveAgents() []*model.Agent {
	return util.FilterAgents(g.Agents, func(agent *model.Agent) bool {
		return g.isAlive(agent)
	})
}

func (g *Game) getAliveWerewolves() []*model.Agent {
	return util.FilterAgents(g.Agents, func(agent *model.Agent) bool {
		return g.isAlive(agent) && agent.Role.Species == model.S_WEREWOLF
	})
}

func (g *Game) isAlive(agent *model.Agent) bool {
	return g.getCurrentGameStatus().StatusMap[*agent] == model.S_ALIVE
}

func (g *Game) getRealtimeBroadcastPacket() model.BroadcastPacket {
	g.realtimeBroadcasterPacketIdx++
	packet := model.BroadcastPacket{
		Id:        g.ID,
		Idx:       g.realtimeBroadcasterPacketIdx,
		Day:       g.currentDay,
		Event:     "なし",
		Message:   nil,
		FromIdx:   nil,
		ToIdx:     nil,
		BubbleIdx: nil,
	}
	for _, agent := range g.Agents {
		packet.Agents = append(packet.Agents, struct {
			Idx     int    `json:"idx"`
			Team    string `json:"team"`
			Name    string `json:"name"`
			Role    string `json:"role"`
			IsAlive bool   `json:"isAlive"`
		}{
			Idx:     agent.Idx,
			Team:    agent.Team,
			Name:    agent.Name,
			Role:    agent.Role.Name,
			IsAlive: g.isAlive(agent),
		})
	}
	return packet
}
