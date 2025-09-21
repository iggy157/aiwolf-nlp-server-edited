package logic

import (
	"fmt"
	"log/slog"

	"github.com/iggy157/aiwolf-nlp-server-edited-edited/model"
)

func (g *Game) doDivine() {
	slog.Info("占いフェーズを開始します", "id", g.id, "day", g.currentDay)
	for _, agent := range g.getAliveAgents() {
		if agent.Role == model.R_SEER {
			g.conductDivination(agent)
			break
		}
	}
	slog.Info("占いフェーズを終了します", "id", g.id, "day", g.currentDay)
}

func (g *Game) conductDivination(agent *model.Agent) {
	slog.Info("占いアクションを開始します", "id", g.id, "agent", agent.String())
	target, err := g.findTargetByRequest(agent, model.R_DIVINE)
	if err != nil {
		slog.Warn("占い対象が見つからなかったため、占い結果を設定しません", "id", g.id)
		return
	}
	if !g.isAlive(target) {
		slog.Warn("占い対象が死亡しているため、占い結果を設定しません", "id", g.id, "target", target.String())
		return
	}
	if agent == target {
		slog.Warn("占い対象が自分自身であるため、占い結果を設定しません", "id", g.id, "target", target.String())
		return
	}
	g.getCurrentGameStatus().DivineResult = &model.Judge{
		Day:    g.getCurrentGameStatus().Day,
		Agent:  *agent,
		Target: *target,
		Result: target.Role.Species,
	}
	if g.gameLogger != nil {
		g.gameLogger.AppendLog(g.id, fmt.Sprintf("%d,divine,%d,%d,%s", g.currentDay, agent.Idx, target.Idx, target.Role.Species))
	}
	if g.realtimeBroadcaster != nil {
		packet := g.getRealtimeBroadcastPacket()
		packet.Event = "占い"
		packet.FromIdx = &agent.Idx
		packet.ToIdx = &target.Idx
		g.realtimeBroadcaster.Broadcast(packet)
	}
	slog.Info("占い結果を設定しました", "id", g.id, "target", target.String(), "result", target.Role.Species)
}
