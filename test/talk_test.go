package test

import (
	"sync"
	"testing"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

func TestTalkPhase1(t *testing.T) {
	t.Log("トークフェーズ")
	config, err := model.LoadFromPath("./config/talk.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	sendMessagesMap := map[string][]string{
		"WEREWOLF":   {"Hello World!"},
		"POSSESSED":  {"Hello World!"},
		"SEER":       {"Hello World!"},
		"VILLAGER-A": {"Hello World!"},
		"VILLAGER-B": {"Hello World!"},
	}
	expectMessagesMap := map[string][]string{
		"WEREWOLF":   {"Hello World!"},
		"POSSESSED":  {"Hello World!"},
		"SEER":       {"Hello World!"},
		"VILLAGER-A": {"Hello World!"},
		"VILLAGER-B": {"Hello World!"},
	}
	executeTalkPhase(t, sendMessagesMap, expectMessagesMap, config)
}

func executeTalkPhase(t *testing.T, sendMessagesMap map[string][]string, expectMessagesMap map[string][]string, config *model.Config) {
	nameMap := make(map[string]string)
	var nameMapMu sync.Mutex

	messageIdxMap := make(map[string]int)
	var messageIdxMapMu sync.Mutex

	handlers := map[model.Request]func(tc TestClient) (string, error){
		model.R_INITIALIZE: func(tc TestClient) (string, error) {
			nameMapMu.Lock()
			nameMap[tc.originalName] = tc.gameName
			nameMapMu.Unlock()
			return "", nil
		},
		model.R_TALK: func(tc TestClient) (string, error) {
			messageIdxMapMu.Lock()
			idx := messageIdxMap[tc.originalName]
			messageIdxMap[tc.originalName]++
			messageIdxMapMu.Unlock()
			message := model.T_OVER
			if idx < len(sendMessagesMap[tc.originalName]) {
				message = sendMessagesMap[tc.originalName][idx]
			}
			tc.t.Logf("トーク: %s < %s", tc.gameName, message)
			return message, nil
		},
		model.R_DAILY_FINISH: func(tc TestClient) (string, error) {
			return "", nil
		},
	}
	executeGame(t, []string{"WEREWOLF", "POSSESSED", "SEER", "VILLAGER-A", "VILLAGER-B"}, config, handlers)
}
