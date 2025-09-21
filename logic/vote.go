package logic

import (
	"fmt"
	"log/slog"

	"github.com/iggy157/aiwolf-nlp-server-edited-edited/model"
)

func (g *Game) executeVote() {
	slog.Info("投票アクションを開始します", "id", g.id, "day", g.currentDay)
	g.getCurrentGameStatus().Votes = g.collectVotes(model.R_VOTE, g.getAliveAgents())
}

func (g *Game) executeAttackVote() {
	slog.Info("襲撃投票アクションを開始します", "id", g.id, "day", g.currentDay)
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
			slog.Warn("投票対象が死亡しているため、投票を無視します", "id", g.id, "agent", agent.String(), "target", target.String())
			continue
		}
		if (request == model.R_VOTE && !g.config.Game.Vote.AllowSelfVote) || (request == model.R_ATTACK && !g.config.Game.AttackVote.AllowSelfVote) {
			if agent.Idx == target.Idx {
				slog.Warn("自己投票は許可されていないため、投票を無視します", "id", g.id, "agent", agent.String(), "target", target.String())
				continue
			}
		}
		votes = append(votes, model.Vote{
			Day:    g.getCurrentGameStatus().Day,
			Agent:  *agent,
			Target: *target,
		})
		if g.gameLogger != nil {
			if request == model.R_VOTE {
				g.gameLogger.AppendLog(g.id, fmt.Sprintf("%d,vote,%d,%d", g.currentDay, agent.Idx, target.Idx))
			} else {
				g.gameLogger.AppendLog(g.id, fmt.Sprintf("%d,attackVote,%d,%d", g.currentDay, agent.Idx, target.Idx))
			}
		}

		if g.realtimeBroadcaster != nil {
			if request == model.R_VOTE {
				packet := g.getRealtimeBroadcastPacket()
				packet.Event = "投票"
				packet.FromIdx = &agent.Idx
				packet.ToIdx = &target.Idx
				g.realtimeBroadcaster.Broadcast(packet)
			} else {
				packet := g.getRealtimeBroadcastPacket()
				packet.Event = "襲撃投票"
				packet.FromIdx = &agent.Idx
				packet.ToIdx = &target.Idx
				g.realtimeBroadcaster.Broadcast(packet)
			}
		}
		slog.Info("投票を受信しました", "id", g.id, "agent", agent.String(), "target", target.String())
	}
	return votes

}
