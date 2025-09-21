package logic

import (
	"fmt"
	"log/slog"

	"github.com/iggy157/aiwolf-nlp-server-edited-edited/model"
	"github.com/iggy157/aiwolf-nlp-server-edited-edited/util"
)

func (g *Game) getAttackVotedCandidates(votes []model.Vote) []model.Agent {
	return util.GetCandidates(votes, func(vote model.Vote) bool {
		return vote.Target.Role != model.R_WEREWOLF
	})
}

func (g *Game) doAttack() {
	slog.Info("襲撃フェーズを開始します", "id", g.id, "day", g.currentDay)
	var attacked *model.Agent
	werewolfs := g.getAliveWerewolves()
	if len(werewolfs) > 0 {
		candidates := make([]model.Agent, 0)
		for range g.setting.AttackVote.MaxCount {
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
				g.gameLogger.AppendLog(g.id, fmt.Sprintf("%d,attack,%d,true", g.currentDay, attacked.Idx))
			}
			if g.realtimeBroadcaster != nil {
				packet := g.getRealtimeBroadcastPacket()
				packet.Event = "襲撃"
				packet.ToIdx = &attacked.Idx
				g.realtimeBroadcaster.Broadcast(packet)
			}
			slog.Info("襲撃結果を設定しました", "id", g.id, "agent", attacked.String())
		} else if attacked != nil {
			if g.gameLogger != nil {
				g.gameLogger.AppendLog(g.id, fmt.Sprintf("%d,attack,%d,false", g.currentDay, attacked.Idx))
			}
			if g.realtimeBroadcaster != nil {
				packet := g.getRealtimeBroadcastPacket()
				packet.Event = "襲撃"
				idx := -1
				packet.FromIdx = &idx
				packet.ToIdx = &attacked.Idx
				g.realtimeBroadcaster.Broadcast(packet)
			}
			slog.Info("護衛されたため、襲撃結果を設定しません", "id", g.id, "agent", attacked.String())
		} else {
			if g.gameLogger != nil {
				g.gameLogger.AppendLog(g.id, fmt.Sprintf("%d,attack,-1,true", g.currentDay))
			}
			if g.realtimeBroadcaster != nil {
				packet := g.getRealtimeBroadcastPacket()
				packet.Event = "襲撃"
				g.realtimeBroadcaster.Broadcast(packet)
			}
			slog.Info("襲撃対象がいないため、襲撃結果を設定しません", "id", g.id)
		}
	}
	slog.Info("襲撃フェーズを終了します", "id", g.id, "day", g.currentDay)
}

func (g *Game) isGuarded(attacked *model.Agent) bool {
	if g.getCurrentGameStatus().Guard == nil {
		return false
	}
	return g.getCurrentGameStatus().Guard.Target == *attacked && g.isAlive(&g.getCurrentGameStatus().Guard.Agent)
}
