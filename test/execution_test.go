package test

import (
	"testing"

	"github.com/aiwolfdial/aiwolf-nlp-server/model"
)

func TestExecutionPhase1(t *testing.T) {
	config, err := model.LoadFromPath("./config/execution.yml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	voteTargets := map[string]string{
		"Player1": "Player5",
		"Player2": "Player1",
		"Player3": "Player1",
		"Player4": "Player1",
		"Player5": "Player1",
	}
	expectStatus := map[string]model.Status{
		"Player1": model.S_DEAD,
		"Player2": model.S_ALIVE,
		"Player3": model.S_ALIVE,
		"Player4": model.S_ALIVE,
		"Player5": model.S_ALIVE,
	}
	executeExecutionPhase(t, voteTargets, expectStatus, config)
}

func executeExecutionPhase(t *testing.T, voteTargets map[string]string, expectStatus map[string]model.Status, config *model.Config) {
	handlers := map[model.Request]func(tc TestClient) (string, error){
		model.R_VOTE: func(tc TestClient) (string, error) {
			if target, exists := voteTargets[tc.name]; exists {
				tc.t.Logf("Voting for %s", target)
				return target, nil
			} else {
				tc.t.Errorf("No vote target defined for %s", tc.name)
				return "", nil
			}
		},
		model.R_FINISH: func(tc TestClient) (string, error) {
			if statusMap, exists := tc.info["status_map"].(map[string]any); exists {
				for k, v := range statusMap {
					if expectedStatus, ok := expectStatus[k]; ok {
						if v != expectedStatus.String() {
							tc.t.Errorf("Expected %s to be %s, but it is %s", k, expectedStatus, v)
						} else {
							tc.t.Logf("Agent %s is %s as expected", k, expectedStatus)
						}
					} else {
						tc.t.Errorf("Unexpected agent %s with status %s", k, v)
					}
				}
				return "", nil
			} else {
				tc.t.Error("status_map not found in info")
			}
			return "", nil
		},
	}
	ExecuteGame(t, config, handlers)
}
