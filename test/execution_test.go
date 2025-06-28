package test

import (
	"testing"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

func TestExecutionPhase1(t *testing.T) {

	t.Log("追放フェーズ: 投票数が最も多いプレイヤーが追放される")
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	targetMap := map[string]string{
		"WEREWOLF":   "VILLAGER-B",
		"POSSESSED":  "WEREWOLF",
		"SEER":       "WEREWOLF",
		"VILLAGER-A": "WEREWOLF",
		"VILLAGER-B": "WEREWOLF",
	}
	expectStatuses := []map[string]model.Status{
		{
			"WEREWOLF":   model.S_DEAD,
			"POSSESSED":  model.S_ALIVE,
			"SEER":       model.S_ALIVE,
			"VILLAGER-A": model.S_ALIVE,
			"VILLAGER-B": model.S_ALIVE,
		},
	}
	executeExecutionPhase(t, targetMap, expectStatuses, config)
}

func TestExecutionPhase2(t *testing.T) {

	t.Log("追放フェーズ: 投票数が同数の場合、ランダムで追放される")
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	targetMap := map[string]string{
		"WEREWOLF":   "VILLAGER-B",
		"POSSESSED":  "WEREWOLF",
		"SEER":       "WEREWOLF",
		"VILLAGER-A": "POSSESSED",
		"VILLAGER-B": "POSSESSED",
	}
	expectStatuses := []map[string]model.Status{
		{
			"WEREWOLF":   model.S_DEAD,
			"POSSESSED":  model.S_ALIVE,
			"SEER":       model.S_ALIVE,
			"VILLAGER-A": model.S_ALIVE,
			"VILLAGER-B": model.S_ALIVE,
		},
		{
			"WEREWOLF":   model.S_ALIVE,
			"POSSESSED":  model.S_DEAD,
			"SEER":       model.S_ALIVE,
			"VILLAGER-A": model.S_ALIVE,
			"VILLAGER-B": model.S_ALIVE,
		},
	}
	executeExecutionPhase(t, targetMap, expectStatuses, config)
}

func TestExecutionPhase3(t *testing.T) {

	t.Log("追放フェーズ: 投票がすべて無効の場合、誰も追放されない")
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	targetMap := map[string]string{
		"WEREWOLF":   "Unknown",
		"POSSESSED":  "Unknown",
		"SEER":       "Unknown",
		"VILLAGER-A": "Unknown",
		"VILLAGER-B": "Unknown",
	}
	expectStatuses := []map[string]model.Status{
		{
			"WEREWOLF":   model.S_ALIVE,
			"POSSESSED":  model.S_ALIVE,
			"SEER":       model.S_ALIVE,
			"VILLAGER-A": model.S_ALIVE,
			"VILLAGER-B": model.S_ALIVE,
		},
	}
	executeExecutionPhase(t, targetMap, expectStatuses, config)
}

func TestExecutionPhase4(t *testing.T) {

	t.Log("追放フェーズ: 自己投票が許可されている場合、自己投票を含むプレイヤーが追放される")
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	targetMap := map[string]string{
		"WEREWOLF":   "WEREWOLF",
		"POSSESSED":  "POSSESSED",
		"SEER":       "Unknown",
		"VILLAGER-A": "Unknown",
		"VILLAGER-B": "Unknown",
	}
	expectStatuses := []map[string]model.Status{
		{
			"WEREWOLF":   model.S_DEAD,
			"POSSESSED":  model.S_ALIVE,
			"SEER":       model.S_ALIVE,
			"VILLAGER-A": model.S_ALIVE,
			"VILLAGER-B": model.S_ALIVE,
		},
		{
			"WEREWOLF":   model.S_ALIVE,
			"POSSESSED":  model.S_DEAD,
			"SEER":       model.S_ALIVE,
			"VILLAGER-A": model.S_ALIVE,
			"VILLAGER-B": model.S_ALIVE,
		},
	}
	executeExecutionPhase(t, targetMap, expectStatuses, config)
}

func TestExecutionPhase5(t *testing.T) {

	t.Log("追放フェーズ: 自己投票が許可されていない場合、自己投票を含まないプレイヤーが追放される")
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}
	config.Game.Vote.AllowSelfVote = false

	targetMap := map[string]string{
		"WEREWOLF":   "WEREWOLF",
		"POSSESSED":  "SEER",
		"SEER":       "Unknown",
		"VILLAGER-A": "Unknown",
		"VILLAGER-B": "Unknown",
	}
	expectStatuses := []map[string]model.Status{
		{
			"WEREWOLF":   model.S_ALIVE,
			"POSSESSED":  model.S_ALIVE,
			"SEER":       model.S_DEAD,
			"VILLAGER-A": model.S_ALIVE,
			"VILLAGER-B": model.S_ALIVE,
		},
	}
	executeExecutionPhase(t, targetMap, expectStatuses, config)
}

func executeExecutionPhase(t *testing.T, targetMap map[string]string, expectStatuses []map[string]model.Status, config *model.Config) {
	nameMap := make(map[string]string)

	handlers := map[model.Request]func(tc TestClient) (string, error){
		model.R_INITIALIZE: func(tc TestClient) (string, error) {
			nameMap[tc.originalName] = tc.gameName
			return "", nil
		},
		model.R_VOTE: func(tc TestClient) (string, error) {
			tc.t.Logf("投票: %s -> %s", tc.gameName, nameMap[targetMap[tc.originalName]])
			return nameMap[targetMap[tc.originalName]], nil
		},
		model.R_FINISH: func(tc TestClient) (string, error) {
			return tc.validateStatusPattern(expectStatuses, nameMap)
		},
	}
	executeGame(t, []string{"WEREWOLF", "POSSESSED", "SEER", "VILLAGER-A", "VILLAGER-B"}, config, handlers)
}
