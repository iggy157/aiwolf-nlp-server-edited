package logic

import (
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"unicode/utf8"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

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
	var talkSetting *model.TalkSetting
	var talkList *[]model.Talk

	switch request {
	case model.R_TALK:
		agents = g.getAliveAgents()
		talkSetting = &g.setting.Talk.TalkSetting
		talkList = &g.getCurrentGameStatus().Talks
	case model.R_WHISPER:
		agents = g.getAliveWerewolves()
		talkSetting = &g.setting.Whisper.TalkSetting
		talkList = &g.getCurrentGameStatus().Whispers
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
		remainCountMap[*agent] = talkSetting.MaxCount.PerAgent
		if talkSetting.MaxLength.PerAgent != nil {
			remainLengthMap[*agent] = *talkSetting.MaxLength.PerAgent
		}
		remainSkipMap[*agent] = talkSetting.MaxSkip
	}
	g.getCurrentGameStatus().RemainCountMap = &remainCountMap
	g.getCurrentGameStatus().RemainLengthMap = &remainLengthMap
	g.getCurrentGameStatus().RemainSkipMap = &remainSkipMap

	rand.Shuffle(len(agents), func(i, j int) {
		agents[i], agents[j] = agents[j], agents[i]
	})

	idx := 0
	for i := range talkSetting.MaxCount.PerDay {
		cnt := false
		for _, agent := range agents {
			if remainCountMap[*agent] <= 0 {
				continue
			}
			if value, exists := remainLengthMap[*agent]; exists {
				if value <= 0 {
					continue
				}
			}
			remainCountMap[*agent]--
			text := g.getTalkWhisperText(agent, request)
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
				remainSkipMap[*agent] = talkSetting.MaxSkip
				slog.Info("発言がオーバーもしくはスキップではないため、スキップ回数をリセットしました", "id", g.ID, "agent", agent.String())
			}
			if talkSetting.MaxLength.PerAgent != nil {
				var length int
				if *talkSetting.MaxLength.CountInWord {
					length = utf8.RuneCountInString(text)
				} else {
					length = len(strings.Fields(text))
				}
				remainLength := *talkSetting.MaxLength.BaseLength + remainLengthMap[*agent]
				if remainLength <= 0 {
					text = model.T_OVER
					slog.Warn("残り文字数がないため、発言をオーバーに置換しました", "id", g.ID, "agent", agent.String())
				} else if length > remainLength {
					text = string([]rune(text)[:remainLength])
					remainLengthMap[*agent] = 0
					slog.Warn("発言が最大文字数を超えたため、切り捨てました", "id", g.ID, "agent", agent.String())
				} else {
					if length > *talkSetting.MaxLength.BaseLength {
						remainLengthMap[*agent] -= length - *talkSetting.MaxLength.BaseLength
					}
				}
			}
			if talkSetting.MaxLength.PerTalk != nil {
				if utf8.RuneCountInString(text) > *talkSetting.MaxLength.PerTalk {
					text = string([]rune(text)[:*talkSetting.MaxLength.PerTalk])
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
					packet.Event = "トーク"
					packet.Message = &talk.Text
					packet.BubbleIdx = &agent.Idx
					g.realtimeBroadcaster.Broadcast(packet)
				} else {
					packet := g.getRealtimeBroadcastPacket()
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
