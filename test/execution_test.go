package test

import (
	"testing"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

func TestExecutionPhase1(t *testing.T) {
	t.Parallel()
	t.Log("追放フェーズ: 投票数が最も多いプレイヤーが追放される")
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	targetNames := map[string]string{
		"Player1": "Player5",
		"Player2": "Player1",
		"Player3": "Player1",
		"Player4": "Player1",
		"Player5": "Player1",
	}
	expectStatuses := []map[string]model.Status{
		{
			"Player1": model.S_DEAD,
			"Player2": model.S_ALIVE,
			"Player3": model.S_ALIVE,
			"Player4": model.S_ALIVE,
			"Player5": model.S_ALIVE,
		},
	}
	executeExecutionPhase(t, targetNames, expectStatuses, config)
}

func TestExecutionPhase2(t *testing.T) {
	t.Parallel()
	t.Log("追放フェーズ: 投票数が同数の場合、ランダムで追放される")
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	targetNames := map[string]string{
		"Player1": "Player5",
		"Player2": "Player1",
		"Player3": "Player1",
		"Player4": "Player2",
		"Player5": "Player2",
	}
	expectStatuses := []map[string]model.Status{
		{
			"Player1": model.S_DEAD,
			"Player2": model.S_ALIVE,
			"Player3": model.S_ALIVE,
			"Player4": model.S_ALIVE,
			"Player5": model.S_ALIVE,
		},
		{
			"Player1": model.S_ALIVE,
			"Player2": model.S_DEAD,
			"Player3": model.S_ALIVE,
			"Player4": model.S_ALIVE,
			"Player5": model.S_ALIVE,
		},
	}
	executeExecutionPhase(t, targetNames, expectStatuses, config)
}

func TestExecutionPhase3(t *testing.T) {
	t.Parallel()
	t.Log("追放フェーズ: 投票がすべて無効の場合、誰も追放されない")
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	targetNames := map[string]string{
		"Player1": "Unknown",
		"Player2": "Unknown",
		"Player3": "Unknown",
		"Player4": "Unknown",
		"Player5": "Unknown",
	}
	expectStatuses := []map[string]model.Status{
		{
			"Player1": model.S_ALIVE,
			"Player2": model.S_ALIVE,
			"Player3": model.S_ALIVE,
			"Player4": model.S_ALIVE,
			"Player5": model.S_ALIVE,
		},
	}
	executeExecutionPhase(t, targetNames, expectStatuses, config)
}

func TestExecutionPhase4(t *testing.T) {
	t.Parallel()
	t.Log("追放フェーズ: 自己投票が許可されている場合、自己投票を含むプレイヤーが追放される")
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	targetNames := map[string]string{
		"Player1": "Player1",
		"Player2": "Player2",
		"Player3": "Unknown",
		"Player4": "Unknown",
		"Player5": "Unknown",
	}
	expectStatuses := []map[string]model.Status{
		{
			"Player1": model.S_DEAD,
			"Player2": model.S_ALIVE,
			"Player3": model.S_ALIVE,
			"Player4": model.S_ALIVE,
			"Player5": model.S_ALIVE,
		},
		{
			"Player1": model.S_ALIVE,
			"Player2": model.S_DEAD,
			"Player3": model.S_ALIVE,
			"Player4": model.S_ALIVE,
			"Player5": model.S_ALIVE,
		},
	}
	executeExecutionPhase(t, targetNames, expectStatuses, config)
}

func TestExecutionPhase5(t *testing.T) {
	t.Parallel()
	t.Log("追放フェーズ: 自己投票が許可されていない場合、自己投票を含まないプレイヤーが追放される")
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}
	config.Game.Vote.AllowSelfVote = false

	targetNames := map[string]string{
		"Player1": "Player1",
		"Player2": "Player3",
		"Player3": "Unknown",
		"Player4": "Unknown",
		"Player5": "Unknown",
	}
	expectStatuses := []map[string]model.Status{
		{
			"Player1": model.S_ALIVE,
			"Player2": model.S_ALIVE,
			"Player3": model.S_DEAD,
			"Player4": model.S_ALIVE,
			"Player5": model.S_ALIVE,
		},
	}
	executeExecutionPhase(t, targetNames, expectStatuses, config)
}

func executeExecutionPhase(t *testing.T, targetNames map[string]string, expectStatuses []map[string]model.Status, config *model.Config) {
	handlers := map[model.Request]func(tc TestClient) (string, error){
		model.R_VOTE: func(tc TestClient) (string, error) {
			if target, exists := targetNames[tc.gameName]; exists {
				tc.t.Logf("投票: %s -> %s", tc.gameName, target)
				return target, nil
			} else {
				tc.t.Errorf("投票対象が見つかりません: %s", tc.gameName)
				return "", nil
			}
		},
		model.R_FINISH: func(tc TestClient) (string, error) {
			if statusMap, exists := tc.info["status_map"].(map[string]any); exists {
				for _, expectStatus := range expectStatuses {
					matchesPattern := true
					for k, expectedStatus := range expectStatus {
						if v, ok := statusMap[k]; ok {
							if v != expectedStatus.String() {
								matchesPattern = false
								break
							}
						} else {
							matchesPattern = false
							break
						}
					}
					if matchesPattern {
						tc.t.Logf("期待されるステータスパターンと一致しました")
						for k, v := range statusMap {
							tc.t.Logf("%s: %s", k, v)
						}
						return "", nil
					}
				}
				tc.t.Errorf("期待されるステータスパターンと一致しません")
				for k, v := range statusMap {
					tc.t.Logf("%s: %s", k, v)
				}
			} else {
				tc.t.Error("status_mapが見つかりません")
			}
			return "", nil
		},
	}
	ExecuteSelfMatchGame(t, config, handlers)
}
