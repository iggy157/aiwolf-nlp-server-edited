package util

import "github.com/kano-lab/aiwolf-nlp-server/model"

func SetTargetIdx(packet *model.BroadcastPacket, agent *model.Agent, target *model.Agent) {
	for i, a := range packet.Agents {
		if a.Idx == agent.Idx {
			packet.Agents[i].TargetIdxs = append(packet.Agents[i].TargetIdxs, target.Idx)
			break
		}
	}
}

func SetBubble(packet *model.BroadcastPacket, agent *model.Agent) {
	for i, a := range packet.Agents {
		if a.Idx == agent.Idx {
			packet.Agents[i].IsBubble = true
			break
		}
	}
}
