package test

import (
	"testing"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

func TestAttackPhase1(t *testing.T) {
	t.Parallel()
	t.Log("襲撃フェーズ: 人狼が狂人を襲撃する")
	config, err := model.LoadFromPath("./config/attack.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	targetMap := map[string]string{
		"WEREWOLF": "POSSESSED",
	}
	expectStatuses := []map[string]model.Status{
		{
			"WEREWOLF":   model.S_ALIVE,
			"POSSESSED":  model.S_DEAD,
			"SEER":       model.S_ALIVE,
			"VILLAGER-A": model.S_ALIVE,
			"VILLAGER-B": model.S_ALIVE,
		},
	}
	executeAttackPhase(t, targetMap, expectStatuses, config)
}

func executeAttackPhase(t *testing.T, targetMap map[string]string, expectStatuses []map[string]model.Status, config *model.Config) {
	nameMap := make(map[string]string)

	handlers := map[model.Request]func(tc TestClient) (string, error){
		model.R_INITIALIZE: func(tc TestClient) (string, error) {
			nameMap[tc.originalName] = tc.gameName
			return "", nil
		},
		model.R_ATTACK: func(tc TestClient) (string, error) {
			tc.t.Logf("襲撃投票: %s -> %s", tc.gameName, nameMap[targetMap[tc.originalName]])
			return nameMap[targetMap[tc.originalName]], nil
		},
		model.R_FINISH: func(tc TestClient) (string, error) {
			if statusMap, exists := tc.info["status_map"].(map[string]any); exists {
				for _, expectStatus := range expectStatuses {
					matchesPattern := true
					for k, expectedStatus := range expectStatus {
						if v, ok := statusMap[nameMap[k]]; ok {
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
	ExecuteGame(t, []string{"WEREWOLF", "POSSESSED", "SEER", "VILLAGER-A", "VILLAGER-B"}, config, handlers)
}
