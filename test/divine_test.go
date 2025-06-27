package test

import (
	"testing"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

func TestDivinePhase1(t *testing.T) {
	t.Parallel()
	t.Log("追放フェーズ: 投票数が最も多いプレイヤーが追放される")
	config, err := model.LoadFromPath("./config/divine.yml")
	if err != nil {
		t.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	executeDivinePhase(t, "WEREWOLF", model.S_WEREWOLF, config)
}

func executeDivinePhase(t *testing.T, divineTarget string, expectSpecies model.Species, config *model.Config) {
	handlers := map[model.Request]func(tc TestClient) (string, error){
		model.R_INITIALIZE: func(tc TestClient) (string, error) {

			return "", nil
		},
		model.R_DIVINE: func(tc TestClient) (string, error) {
			return divineTarget, nil
		},
		model.R_DAILY_FINISH: func(tc TestClient) (string, error) {
			if tc.role != model.R_SEER {
				return "", nil
			}
			return "", nil
		},
	}
	ExecuteGame(t, config, handlers)
}
